package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "flag"
)

func check(err error) {
    if err != nil {
        fmt.Printf("ERROR: %s", err);
        os.Exit(-1);
    }
}

func main(){
	var original_file []byte;
	var new_file []byte;
	var differences int = 0;

    original_file_path_ptr := flag.String("op", "", "Path to original path");
    new_file_path_ptr := flag.String("dp", "", "Path to decoded file");

    flag.Parse()

    if *original_file_path_ptr=="" || *new_file_path_ptr==""{
        flag.PrintDefaults()
        os.Exit(1)
    }

    var original_file_path string = *original_file_path_ptr;
    var new_file_path string = *new_file_path_ptr;

	original_file, err := ioutil.ReadFile(original_file_path);
    check(err);

    new_file, err = ioutil.ReadFile(new_file_path);
    check(err);

    if len(new_file)==len(original_file){

    	for a:=0; a<len(original_file);a++{

    		if new_file[a]!=original_file[a]{
    			differences=differences+1
    		}

    	}

    	fmt.Printf("This files have %d differences in a total of %d bytes. It is a %f/100", differences,len(original_file), float32(differences)*100/float32(len(original_file)))

    }else{
    	fmt.Printf("This files do not have the same size! Original: %d New: %d", len(original_file), len(new_file))
    }
}