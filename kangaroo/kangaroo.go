package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "encoding/binary"
    "flag"
)

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

    //G := [4][7]byte{{1,1,1,0,0,0,0}, {1,0,0,1,1,0,0}, {0,1,0,1,0,1,0}, {1,1,0,1,0,0,1}};
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

    new_position, err = binary_file.Seek(offset, 0)
    check(err)

    if new_position==last_position{//check if we reached end of video
        end_of_file=true
        return read_bytes, end_of_file
    }

    _, err = binary_file.Read(read_bytes)
    check(err)
    //fmt.Printf("%d bytes: ", n1)

    if clear_option==1{
        for i:=0; i<len(read_bytes); i++{
            clear_bit(&read_bytes[i], 1)//poner a 0 los valores descartables
            clear_bit(&read_bytes[i], 2)
            clear_bit(&read_bytes[i], 3)
            clear_bit(&read_bytes[i], 4)
            clear_bit(&read_bytes[i], 5)
        }
    }


    return read_bytes, end_of_file
}

func level_1(frame_data []byte, secret_file_bits []byte, y_size int, width int){//changes the LSB of every U and V

    var frame_position int = y_size;//start after Y
    var in_line_position int = 0;

    output_file, err := os.OpenFile("encoded.yuv420", os.O_APPEND|os.O_WRONLY, 0600);
    check(err);
    defer output_file.Close();

    for i:=0; i<len(secret_file_bits); i++{
        
        if secret_file_bits[i]==0{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{
                    frame_data[frame_position+line_block+line_position+in_line_position]=frame_data[frame_position+line_block+line_position+in_line_position]+0x08;
                    //fmt.Printf("Writing: 0 in position: %d \n", frame_position+line_block+line_position+in_line_position)
                }
            }
        }

        if secret_file_bits[i]==1{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{
                    frame_data[frame_position+line_block+line_position+in_line_position]=frame_data[frame_position+line_block+line_position+in_line_position]+0x17;
                    //fmt.Printf("Writing: 1 in position: %d \n", frame_position+line_block+line_position+in_line_position)
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

    secret_file_path_ptr := flag.String("sp", "", "Absolute path to the file we want to hide.");
    video_path_ptr := flag.String("vp", "", "Absolute path to the raw video that will contain the secret file.");
    width_ptr := flag.Int("width", 0, "Video width");
    high_ptr := flag.Int("high", 0, "Video high");
    flag.Parse()

    if *secret_file_path_ptr=="" || *video_path_ptr=="" || *width_ptr==0 || *high_ptr==0 {
        flag.PrintDefaults()
        os.Exit(1)
    }

    var secret_file_path string = *secret_file_path_ptr;
    var video_path string = *video_path_ptr;
    var width int = *width_ptr;
    var high int = *high_ptr;

    var y_size int = width*high; 
    var u_size int = width*high*2/8; //in yuv420 u_size = y_size*2/8 bytes
    var v_size int = u_size;
    var frame_size int = y_size + u_size + v_size;
    var secret_bits_per_frame int = u_size*2/16;//28800

    var frame_data [] byte;

    var secret_file_bits []byte;
    var secret_file_bits_hamming []byte;
    var number_of_bits_hamming [] byte;
    var hamming_block []byte;
    var payload_bits []byte;
    var bit_array []byte;//28 800 bits for this case

    var end_of_video bool = false;
    var end_of_secret bool = false;

    var frame_count int = 0;
    var secret_count int = 0;

    empty:=[]byte("")
    err := ioutil.WriteFile("encoded.yuv420", empty, 0644)
    check(err)

    secret_file, err := ioutil.ReadFile(secret_file_path);
    check(err);

    secret_file_bits=get_bits(secret_file);


    //add hamming to secret bits
    for i:=0; i<len(secret_file_bits); i+=4{
        hamming_block=hamming_encode(secret_file_bits[i:i+4]);
        secret_file_bits_hamming=append(secret_file_bits_hamming, hamming_block...);
    }

    fmt.Printf("\nTotal size: %d",len(secret_file_bits_hamming)+64+16*3)

    //count how many bits we are going to encode in the video
    number_of_bits := make([]byte, 8)
    binary.BigEndian.PutUint64(number_of_bits, uint64(len(secret_file_bits_hamming)+64+16*3))//64 bits for size header + 16*3 bits for size header hamming

    //add hamming to size header
    number_of_bits=get_bits(number_of_bits)

    for i:=0; i<len(number_of_bits); i+=4{
        hamming_block=hamming_encode(number_of_bits[i:i+4]);
        number_of_bits_hamming=append(number_of_bits_hamming, hamming_block...);//+3 or +4?
    }

    //append size and secret
    payload_bits=append(payload_bits, number_of_bits_hamming...)
    payload_bits=append(payload_bits, secret_file_bits_hamming...)




    for end_of_video!=true{

        if end_of_secret!=true{//define frame data and secret data, and give it to level_1 frame by frame

            frame_data, end_of_video=read_frame(video_path, int64(frame_count)*int64(frame_size), 1, frame_size);//read new frame

            if secret_count+secret_bits_per_frame<len(payload_bits){//check if we are in the end of the file

                bit_array=payload_bits[secret_count:secret_count+secret_bits_per_frame]//read the data to hide

            }else{
                bit_array=payload_bits[secret_count:]//maybe this does not work
                end_of_secret=true
            }

            level_1(frame_data, bit_array, y_size, width)

            secret_count=secret_count+secret_bits_per_frame

        }else{
            frame_data, end_of_video=read_frame(video_path, int64(frame_count)*int64(frame_size), 0, frame_size);//read new frame

            if end_of_video!=true{
                output_file, err := os.OpenFile("encoded.yuv420", os.O_APPEND|os.O_WRONLY, 0600);
                check(err);
                defer output_file.Close();

                _, err = output_file.Write(frame_data);//append modified frame to output file
                check(err);

                output_file.Close();
            }
        }
        frame_count=frame_count+1
    }
}