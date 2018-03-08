package main

import (
    "fmt"
    "os"
    "flag"
)

func check(err error) {
    if err != nil {
        fmt.Printf("ERROR: %s", err);
        os.Exit(-1);
    }
}


func read_bytes(file *os.File, offset int64) []byte{
    read_bytes := make([]byte, 1)

    _, err := file.Seek(offset, 0)
    check(err)

    _, err = file.Read(read_bytes)
    check(err)
    return read_bytes
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

func main(){

    file1_path_ptr := flag.String("op", "", "Path to original path");
    file2_path_ptr := flag.String("dp", "", "Path to decoded file");

    flag.Parse();

    if *file1_path_ptr=="" || *file2_path_ptr==""{
        flag.PrintDefaults();
        os.Exit(1);
    }

    var file1_path string = *file1_path_ptr;
    var file2_path string = *file2_path_ptr;

    var bytes1 []byte;
    var bytes2 []byte;

    var bits1 []byte;
    var bits2 []byte;

    var differences [8]int64;
    var similarities [8]int64;

    var total_differences int64;

    //Open file 1 and get info
    file1, err := os.Open(file1_path);
    check(err);
    defer file1.Close();
    info1, err := file1.Stat();
    check(err);

    //Open file 2 and get info
    file2, err := os.Open(file2_path);
    check(err);
    defer file2.Close();
    info2, err := file2.Stat();
    check(err);

    if info1.Size()==info2.Size(){
        var byte_position int64=0;
        
        //read files and compare bit by bit
        for byte_position<info1.Size(){
            bytes1=read_bytes(file1, byte_position);
            bytes2=read_bytes(file2, byte_position);

            bits1=get_bits(bytes1)
            bits2=get_bits(bytes2)

            for b:=0; b<8; b++{

                if bits1[b]!=bits2[b]{
                    differences[b]=differences[b]+1
                }else{
                    similarities[b]=similarities[b]+1
                }
            }
            
            //show results
            fmt.Println("\n")
            for i:=0; i<8; i++{
                total_differences=total_differences+differences[i]
                fmt.Printf("\nBit %d: %d differences, %d total, %E/100 error", i, differences[i], similarities[i]+differences[i], float32(differences[i]*100)/float32((similarities[i]+differences[i])))
            }
            byte_position=byte_position+1;
        }

        fmt.Printf("Total differences: %d ---> %d/100", total_differences, total_differences/info1.Size())


    }else{
        fmt.Println("This files do not have the same size !")
        fmt.Printf("File 1: %d bytes", info1.Size())
        fmt.Printf("\nFile 2: %d bytes", info2.Size())
    }
}