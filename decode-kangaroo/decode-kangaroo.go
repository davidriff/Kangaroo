package main

import (
    "fmt"
    "os"
    "math"
    "encoding/binary"
    "io/ioutil"
    "flag"
)

func check(err error) {
    if err != nil {
        fmt.Printf("ERROR: %s", err);
        os.Exit(-1);
    }
}

func read_frame(file_path string, offset int64, frame_size int) ([]byte, bool) {

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

    //fmt.Println(end_of_file)

    _, err = binary_file.Read(read_bytes)
    check(err)
    return read_bytes, end_of_file
}

func clear_bit(byte_in *byte, position int) { //sets to 0 the specified bit, call it by clear_bit(&variable, bit_position)
    dummy_values := [8]byte{0xfe, 0xfd, 0xfb, 0xf7, 0xef, 0xdf, 0xbf, 0x7f}
    out := *byte_in & dummy_values[position-1]
    *byte_in = out
}

func bits_to_byte(bit_array [8]byte, order int) byte{// order specifies if the bits are LittleEndian or BigEndian
    var byte_out byte = 0;
    var bit_values []byte;

    if order==0{
        bit_values=[]byte{128,64,32,16,8,4,2,1}
    }else{
        bit_values=[]byte{1,2,4,8,16,32,64,128}
    }

    for b:=0; b<8; b++{
        if bit_array[b]==1{
            byte_out=byte_out+bit_values[b];//b or 8-b ??
        }
    }
    return byte_out
}

func hamming_decode (input_array []byte) []byte {

    var syndrome [3]byte;
    var error_position byte;
    var result byte;
    var output []byte;

    H:=[][]byte{{1,0,0}, {0,1,0}, {1,1,0}, {0,0,1}, {1,0,1}, {0,1,1},{1,1,1}}

    R:=[][]byte{{0,0,0,0}, {0,0,0,0}, {1,0,0,0}, {0,0,0,0}, {0,1,0,0}, {0,0,1,0}, {0,0,0,1}}

    for i:=0; i<3; i++{
        result=0;
        for j:=0; j<7; j++{
            result = result^input_array[j]&H[j][i]
        }
        syndrome[i]=result
    }

    if syndrome[0]!=0 || syndrome[1]!=0 || syndrome[2]!=0{

        error_position = syndrome[0]*byte(1)+syndrome[1]*byte(2)+syndrome[2]*byte(4)

        if input_array[error_position-1]==0{//emulates NOT logical operator for byte type
            input_array[error_position-1]=1
        }else{
            input_array[error_position-1]=0
        }
        fmt.Printf("Error detected at position: %d\n", error_position)
    }

    for i:=0; i<4; i++{
        result=0
        for j:=0;j<7; j++{
            result=result^input_array[j]&R[j][i]
        }
        output=append(output, result)
    }
    return output
}

func extract_bits(number_of_bits_to_read uint64, encoded_path string, frame_size int, y_size int, width int) []byte{//reads frames and extracts secret bits until the number is reached

    var frame_data []byte;
    var output_data []byte;
    var output_data_decoded []byte;
    var sum int = 0;
    var mean float64;
    var actual_byte byte;

    var end_of_frame bool = false;

    var frame_count int = 0;
    var frame_position int;//start after Y
    var in_line_position int = 0;
    var extracted_bits uint64 = 0;

    var try_23 float64;
    var try_8 float64;

    for extracted_bits!=number_of_bits_to_read{

        end_of_frame=false;
        frame_position= y_size;
        frame_data, _=read_frame(encoded_path, int64(frame_count)*int64(frame_size), frame_size);//load frame
        

        for end_of_frame!=true{

            for line_position:=0; line_position<width; line_position+=width/4{
                for line_block:=0; line_block<4; line_block++{

                    actual_byte=frame_data[frame_position+line_block+line_position+in_line_position]
                    
                    clear_bit(&actual_byte, 6);
                    clear_bit(&actual_byte, 7);
                    clear_bit(&actual_byte, 8);

                    sum=sum+int(actual_byte);
                }
            }
            in_line_position=in_line_position+4;
            

            if in_line_position%(width/4)==0{//we reached the end of block-line, jump to next block-line (one block-line is made of 4 rows (320*4))
                in_line_position=0;
                frame_position=frame_position+width;
            }

            mean=float64(sum)/16;
            sum=0;

            try_23 = math.Abs(float64(mean)-float64(23));//calculate if encoded bit is 1 or 0
            try_8 = math.Abs(float64(mean)-float64(8));

            if try_23 < try_8 {
                output_data=append(output_data, 1);
                
            }else{
                output_data=append(output_data, 0);
            }

            extracted_bits=extracted_bits+1;

            if frame_position==frame_size{//end of frame, go to next frame
                end_of_frame=true;
                frame_count=frame_count+1;
            }
            if extracted_bits==number_of_bits_to_read{//we read all we needed
                break;
            }
        }
    }

    for i:=0; i<len(output_data); i+=7{
        output_data_decoded=append(output_data_decoded, hamming_decode(output_data[i:i+7])...)
    }

    return output_data_decoded
}

func get_secret_size(encoded_path string, frame_size int, y_size int, width int) uint64{

    var secret_size uint64;
    var secret_size_in_bits []byte;
    var dummy_slice [] byte;
    var dummy_array [8]byte;

    var secret_size_in_bytes  []byte;

    secret_size_in_bits = extract_bits(64+16*3, encoded_path, frame_size, y_size, width);//64+16*3 for the hamming bits of size header

    //reverse each bit inside each byte
    for b:=0; b<8; b++{
        dummy_slice=secret_size_in_bits[b*8:b*8+8];

        for a:=0; a<8; a++{//we need to use an array, or the original slice will be modified with each iteration
            dummy_array[a]=dummy_slice[a];
        }

        for i:=0; i<8; i++{
            secret_size_in_bits[b*8+i]=dummy_array[7-i];
        }
    }

    // turn to bytes
    for c:=0; c<8; c++{
        dummy_slice=secret_size_in_bits[c*8:c*8+8];

        for a:=0; a<8; a++{//we need to use an array, or the original slice will be modified with each iteration
            dummy_array[a]=dummy_slice[a];
        }

        secret_size_in_bytes=append(secret_size_in_bytes, bits_to_byte(dummy_array,0));
    }

    //turn to uint64
    secret_size=binary.BigEndian.Uint64(secret_size_in_bytes);

    fmt.Printf("\n%d bits to read", secret_size)
    return secret_size
}

func main() {

    encoded_file_path_ptr := flag.String("i", "", "Path to encoded file");
    output_path_ptr := flag.String("o", "decoded.file", "Path for output file");
    width_ptr := flag.Int("w", 0, "Video width");
    high_ptr := flag.Int("h", 0, "Video high");
    flag.Parse();

    if *encoded_file_path_ptr=="" || *width_ptr==0 || *high_ptr==0 {
        flag.PrintDefaults();
        os.Exit(1);
    }

    //constants of this video
    var encoded_path string = *encoded_file_path_ptr;
    var decoded_path string = *output_path_ptr;
    var width int = *width_ptr;
    var high int = *high_ptr;
    var y_size int = width*high; 
    var u_size int = width*high*2/8; //in yuv420 u_size = y_size*2/8 bytes
    var v_size int = u_size;
    var frame_size int = y_size + u_size + v_size;
    //

    var secret_in_bits []byte;
    var secret_bit_array [8]byte;
    var secret_in_bytes []byte;

    var secret_size uint64=get_secret_size(encoded_path, frame_size, y_size, width);

    //read the secret
    fmt.Println("\n\nReading...");
    secret_in_bits=extract_bits(secret_size, encoded_path, frame_size, y_size, width);
    fmt.Printf("\n%d bits recovered", len(secret_in_bits));

    for i:=64; i<len(secret_in_bits); i+=8{//turn bits to bytes(skip the first 64 bits which are used for secret size)

        for a:=0; a<8; a++{
            secret_bit_array[a]=secret_in_bits[i+a];
        }
        secret_in_bytes=append(secret_in_bytes, bits_to_byte(secret_bit_array,1));

    }
    err := ioutil.WriteFile(decoded_path, secret_in_bytes, 0644);
    check(err);
 }