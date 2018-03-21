package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
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
    frame_percentage_ptr := flag.Int("frame", 100, "Percentage of frames to use:10, 25, 50 or 100")
    bits_to_use_ptr := flag.Int("bits", 4, "How many bits to use in each byte: 1-8")
    flag.Parse();

    if *secret_file_path_ptr==""{
        fmt.Println("Please, enter a file to secret path\n")
        flag.PrintDefaults();
        os.Exit(1);
    }

    if *video_path_ptr==""{
        fmt.Println("Please, enter a file to video path\n")
        flag.PrintDefaults();
        os.Exit(1);
    }

    if *width_ptr<=0{
        fmt.Println("Please, enter a valid width value\n")
        flag.PrintDefaults();
        os.Exit(1);
    }

    if *high_ptr<=0{
        fmt.Println("Please, enter a valid high value\n")
        flag.PrintDefaults();
        os.Exit(1);
    }

    if *yuv_option_ptr<0 || *yuv_option_ptr>4 {
        fmt.Println("Please, enter a valid value for yuv parameter\n")
        flag.PrintDefaults();
        os.Exit(1);
    }

    if *bits_to_use_ptr<1 || *bits_to_use_ptr>8{
        fmt.Println("Please, enter a valid value for bits parameter\n")
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
        fmt.Println("Please, enter a valid value for frame percentage parameter\n")
        flag.PrintDefaults();
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

func runcmd(cmd string) []byte {

    out, err := exec.Command("bash", "-c", cmd).Output()
    if err != nil {
        fmt.Println("\nERROR executing command: "+cmd)
    }
    return out
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

func encode_ldpc(file_in_bits []byte) []byte{

    var encoded_file_bits []byte;
    var file_bits_in_ascii []byte;

    for i:=0; i<len(file_in_bits); i++{

        if file_in_bits[i] == 0{
            file_bits_in_ascii=append(file_bits_in_ascii, 48);

        }else if file_in_bits[i]==1{
            file_bits_in_ascii=append(file_bits_in_ascii, 49);
        }
    }

    err := ioutil.WriteFile("files/file_bits_in_ascii", file_bits_in_ascii, 0644);
    check(err);

    runcmd("encode files/ldpc.pchk files/ldpc.gen files/file_bits_in_ascii files/encoded")

    encoded_file_ascii, err := ioutil.ReadFile("files/encoded");

    for i:=0; i<len(encoded_file_ascii); i++{

        if encoded_file_ascii[i]==48{
            encoded_file_bits=append(encoded_file_bits,0)

        }else if encoded_file_ascii[i]==49{
            encoded_file_bits=append(encoded_file_bits,1)
        }
    }

    return encoded_file_bits
}

func read_frame(file_path string, offset int64, clear_option byte, frame_size int, bits_to_use int) ([]byte, bool) {

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
            for b:=0; b<bits_to_use; b++{
                clear_bit(&read_bytes[i], b+1)
            }
        }
    }


    return read_bytes, end_of_file
}

func embed(frame_data []byte, secret_file_bits []byte, start_position int, width int, output_path string, one_value byte, zero_value byte){//changes the LSB of every U and V

    var frame_position int = start_position;//start at Y, U or V
    var in_line_position int = 0;

    output_file, err := os.OpenFile(output_path, os.O_APPEND|os.O_WRONLY, 0600);
    check(err);
    defer output_file.Close();

    for i:=0; i<len(secret_file_bits); i++{
        
        if secret_file_bits[i]==0{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{
                    frame_data[frame_position+line_block+line_position+in_line_position]=frame_data[frame_position+line_block+line_position+in_line_position]+zero_value;
                }
            }
        }

        if secret_file_bits[i]==1{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{
                    frame_data[frame_position+line_block+line_position+in_line_position]=frame_data[frame_position+line_block+line_position+in_line_position]+one_value;
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

    var y_size int = width*high; 
    var u_size int = width*high*2/8; //in yuv420 u_size = y_size*2/8 bytes
    var v_size int = u_size;
    var frame_size int = y_size + u_size + v_size;

    //get one_value and zero_value depending on bits_to_use user parameter
    zero_values := []byte{0x00, 0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40}//0, 1, 2, 4, 8, 16 , 32, 64
    one_values := []byte{0x01, 0x02, 0x05, 0x0b, 0x17, 0x2f, 0x5f, 0xbf}//1, 2, 5, 11, 23, 47, 95, 191
    var zero_value byte = zero_values[bits_to_use-1];
    var one_value byte = one_values[bits_to_use-1];

    //every secret bit is encoded in a 4*4 byte bloc (16 bytes)
    yuv_options_size_array := []int{y_size/16, u_size/16, v_size/16, 2*u_size/16, frame_size/16}//indicates how many bits we can embed in each frame depending on yuv_option ex-> for yuv_option 3: u_size*2/16=28800
    yuv_options_start_position_array := []int{0, y_size, y_size+u_size, y_size, 0}//indicates start position for writing (embeding secret) in frame, dependending on yuv options
    
    var secret_bits_per_frame int = yuv_options_size_array[yuv_option];
    var start_position int = yuv_options_start_position_array[yuv_option];

    var frame_data [] byte;

    var secret_file_bits_ldpc []byte;
    var number_of_bits_ldpc []byte;
    var payload_bits []byte;
    var bit_array []byte;

    var end_of_video bool = false;
    var end_of_secret bool = false;

    var frame_count int = 0;
    var secret_count int = 0;

    empty:=[]byte("");
    err := ioutil.WriteFile(output_path, empty, 0644);
    check(err);

    //encode file with ldpc
    runcmd("make-ldpc files/ldpc.pchk 9 10 1 evenboth 3");//build parity check matrix
    runcmd("make-gen files/ldpc.pchk files/ldpc.gen dense");//build generator matrix
    
    secret_file, err := ioutil.ReadFile(secret_file_path);//read secret file
    secret_file_in_bits := get_bits(secret_file);//convert secret file into bits

    secret_file_bits_ldpc = encode_ldpc(secret_file_in_bits);

    fmt.Printf("\nTotal size of payload (in bits): %d\n",len(secret_file_bits_ldpc)+64*10);

    //count how many bits we are going to encode in the video
    number_of_bits := make([]byte, 8);
    binary.BigEndian.PutUint64(number_of_bits, uint64(len(secret_file_bits_ldpc)+64*10));//64 bits for size header * 100 because of ldpc encode

    number_of_bits=get_bits(number_of_bits);

    //encode message size
    number_of_bits_ldpc=encode_ldpc(number_of_bits);

    //append size and secret
    payload_bits=append(payload_bits, number_of_bits_ldpc...);
    payload_bits=append(payload_bits, secret_file_bits_ldpc...);

    for end_of_video!=true{

        if end_of_secret!=true && frame_count%frame_increase==0{//check frame_percentage parameter and define frame data and secret data, and give it to embed() frame by frame

            frame_data, end_of_video=read_frame(video_path, int64(frame_count)*int64(frame_size), 1, frame_size, bits_to_use);//read new frame

            if secret_count+secret_bits_per_frame<len(payload_bits){//check if we are in the end of the file

                bit_array=payload_bits[secret_count:secret_count+secret_bits_per_frame];//read the data to hide

            }else{
                bit_array=payload_bits[secret_count:];
                end_of_secret=true;
            }

            embed(frame_data, bit_array, start_position, width, output_path, one_value, zero_value);

            secret_count=secret_count+secret_bits_per_frame;

        }else{
            frame_data, end_of_video=read_frame(video_path, int64(frame_count)*int64(frame_size), 0, frame_size, bits_to_use);//read new frame

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