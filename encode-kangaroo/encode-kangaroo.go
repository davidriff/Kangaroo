package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "encoding/binary"
    "flag"
)

func get_parameters()(string, string, string, int, int, int, int, int){
    secret_file_path_ptr := flag.String("i", "", "Absolute path to the file we want to hide");
    video_path_ptr := flag.String("v", "", "Absolute path to the raw video (in yuv 420 format) that will contain the secret file");
    output_path_ptr := flag.String("o", "encoded.yuv420", "Path and name for output file");
    width_ptr := flag.Int("w", 0, "Video width");
    high_ptr := flag.Int("h", 0, "Video high");
    yuv_option_ptr := flag.Int("yuv", 3, "Use Luma Y (0), blue chroma Cb (1), red chroma Cr (2), both chromas (3) or all YCbCr (4)")
    frame_percentage_ptr := flag.Int("frame", 100, "Use 10, 25, 50 or 100 percent of all frames")
    bits_to_use_ptr := flag.Int("b", 4, "How many bits to use in each byte")
    flag.Parse();

    if *secret_file_path_ptr=="" || *video_path_ptr=="" || *width_ptr==0 || *high_ptr==0 {
        flag.PrintDefaults();
        os.Exit(1);
    }

    var secret_file_path string = *secret_file_path_ptr;
    var video_path string = *video_path_ptr;
    var output_path string = *output_path_ptr;
    var width int = *width_ptr;
    var high int = *high_ptr;
    var yuv_option int = *yuv_option_ptr;
    var frame_percentage int = *frame_percentage_ptr;
    var bits_to_use int = *bits_to_use_ptr;
    var frame_increase int;

    if frame_percentage==10{
        frame_increase=10;
    }else if frame_percentage==25{
        frame_increase=4;
    }else if frame_percentage==50{
        frame_increase=2;
    }else if frame_percentage==100{
        frame_increase=1;
    }else{
        fmt.Println("You have entered a not allowed percentage of frames.")
        os.Exit(1);
    }

    return secret_file_path, video_path, output_path, width, high, yuv_option, frame_increase, bits_to_use
}

func check(err error) {
    if err != nil {
        fmt.Println(err);
        os.Exit(-1);
    }
}

func get_bits(bytes_in []byte) []byte { //returns the bits in a byte

    var bit_slice []byte;
    var turn byte;

    for b:=0; b<len(bytes_in); b++{
        for i := uint(0); i < 8; i++ {
            turn=bytes_in[b] & (1 << i) >> i
            bit_slice=append(bit_slice, turn)
        }
    }
    return bit_slice
}

func clear_bit(byte_in *byte, position int) { //sets to 0 the specified bit, call it by clear_bit(&variable, bit_position)
    dummy_values := [8]byte{0xfe, 0xfd, 0xfb, 0xf7, 0xef, 0xdf, 0xbf, 0x7f}
    out := *byte_in & dummy_values[position-1]
    *byte_in = out
}

func hamming_encode (input_array []byte) []byte {

    var output []byte;
    var result byte;

    G := [][]byte{{1,1,1,0,0,0,0}, {1,0,0,1,1,0,0}, {0,1,0,1,0,1,0}, {1,1,0,1,0,0,1}};

    for i:=0; i<7; i++{
        result=0;
        for j:=0; j<4; j++{
            result=result^input_array[j]&G[j][i];
        }
        output=append(output, result);
    }
    return output
}

func read_frame(file_path string, offset int64, clear_option byte, frame_size int) ([]byte, bool) {

    var new_position int64;
    var last_position int64;
    var end_of_file bool = false;

    read_bytes := make([]byte, frame_size)

    binary_file, err := os.Open(file_path)
    check(err)
    defer binary_file.Close()

    last_position, err = binary_file.Seek(0,2)
    check(err)

    if offset>last_position{
        end_of_file=true
        return read_bytes, end_of_file
    }

    new_position, err = binary_file.Seek(offset, 0)
    check(err)

    if new_position==last_position{//check if we reached end of video
        end_of_file=true
        return read_bytes, end_of_file
    }

    _, err = binary_file.Read(read_bytes)
    check(err)

    if clear_option==1{
        for i:=0; i<len(read_bytes); i++{
            clear_bit(&read_bytes[i], 1)
            clear_bit(&read_bytes[i], 2)
            clear_bit(&read_bytes[i], 3)
            clear_bit(&read_bytes[i], 4)
            clear_bit(&read_bytes[i], 5)
        }
    }


    return read_bytes, end_of_file
}

func embed(frame_data []byte, secret_file_bits []byte, start_position int, width int, output_path string){//changes the LSB of every U and V

    var frame_position int = start_position;//start at Y, U or V
    var in_line_position int = 0;

    output_file, err := os.OpenFile(output_path, os.O_APPEND|os.O_WRONLY, 0600);
    check(err);
    defer output_file.Close();

    for i:=0; i<len(secret_file_bits); i++{
        
        if secret_file_bits[i]==0{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{
                    frame_data[frame_position+line_block+line_position+in_line_position]=frame_data[frame_position+line_block+line_position+in_line_position]+0x08;
                }
            }
        }

        if secret_file_bits[i]==1{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{
                    frame_data[frame_position+line_block+line_position+in_line_position]=frame_data[frame_position+line_block+line_position+in_line_position]+0x17;
                }
            }
        }

        in_line_position=in_line_position+4

        if in_line_position%(width/4)==0{//we reached the end of block-line, jump to next block-line (one block-line is made of 4 rows (320*4))
            in_line_position=0
            frame_position=frame_position+width
        }
    }
    _, err = output_file.Write(frame_data);//append modified frame to output file
    check(err);
}

func main() {

    var(
        secret_file_path, video_path, output_path, width, high, yuv_option, frame_increase, bits_to_use = get_parameters();
        )

    fmt.Printf("\n Using %d bits of each byte.", bits_to_use)
    var y_size int = width*high; 
    var u_size int = width*high*2/8; //in yuv420 u_size = y_size*2/8 bytes
    var v_size int = u_size;
    var frame_size int = y_size + u_size + v_size;

    //every secret bit is encoded in a 4*4 byte bloc (16 bytes)
    yuv_options_size_array := []int{y_size/16, u_size/16, v_size/16, 2*u_size/16, frame_size/16}//indicates how many bits we can embed in each frame depending on yuv_option ex-> for yuv_option 3: u_size*2/16=28800
    yuv_options_start_position_array := []int{0, y_size, y_size+u_size, y_size, 0}//indicates start position for writing (embeding secret) in frame, dependending on yuv options
    var secret_bits_per_frame int = yuv_options_size_array[yuv_option];
    var start_position int = yuv_options_start_position_array[yuv_option];

    var frame_data [] byte;

    var secret_file_bits []byte;
    var secret_file_bits_hamming []byte;
    var number_of_bits_hamming [] byte;
    var hamming_block []byte;
    var payload_bits []byte;
    var bit_array []byte;

    var end_of_video bool = false;
    var end_of_secret bool = false;

    var frame_count int = 0;
    var secret_count int = 0;

    empty:=[]byte("");
    err := ioutil.WriteFile(output_path, empty, 0644);
    check(err);

    secret_file, err := ioutil.ReadFile(secret_file_path);
    check(err);

    secret_file_bits=get_bits(secret_file);


    //add hamming to secret bits
    for i:=0; i<len(secret_file_bits); i+=4{
        hamming_block=hamming_encode(secret_file_bits[i:i+4]);
        secret_file_bits_hamming=append(secret_file_bits_hamming, hamming_block...);
    }

    fmt.Printf("\nTotal size of payload (in bits): %d",len(secret_file_bits_hamming)+64+16*3);

    //count how many bits we are going to encode in the video
    number_of_bits := make([]byte, 8);
    binary.BigEndian.PutUint64(number_of_bits, uint64(len(secret_file_bits_hamming)+64+16*3));//64 bits for size header + 16*3 bits for size header hamming

    //add hamming to size header
    number_of_bits=get_bits(number_of_bits);

    for i:=0; i<len(number_of_bits); i+=4{
        hamming_block=hamming_encode(number_of_bits[i:i+4]);
        number_of_bits_hamming=append(number_of_bits_hamming, hamming_block...);//+3 or +4?
    }

    //append size and secret
    payload_bits=append(payload_bits, number_of_bits_hamming...);
    payload_bits=append(payload_bits, secret_file_bits_hamming...);

    for end_of_video!=true{

        if end_of_secret!=true && frame_count%frame_increase==0{//define frame data and secret data, and give it to embed() frame by frame

            frame_data, end_of_video=read_frame(video_path, int64(frame_count)*int64(frame_size), 1, frame_size);//read new frame

            if secret_count+secret_bits_per_frame<len(payload_bits){//check if we are in the end of the file

                bit_array=payload_bits[secret_count:secret_count+secret_bits_per_frame];//read the data to hide

            }else{
                bit_array=payload_bits[secret_count:];
                end_of_secret=true;
            }

            embed(frame_data, bit_array, start_position, width, output_path);

            secret_count=secret_count+secret_bits_per_frame;

        }else{
            frame_data, end_of_video=read_frame(video_path, int64(frame_count)*int64(frame_size), 0, frame_size);//read new frame

            if end_of_video!=true{
                output_file, err := os.OpenFile(output_path, os.O_APPEND|os.O_WRONLY, 0600);
                check(err);
                defer output_file.Close();

                _, err = output_file.Write(frame_data);//append modified frame to output file
                check(err);

                output_file.Close();
            }
        }
        frame_count=frame_count+1;
    }
}