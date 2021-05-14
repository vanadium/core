// +build ignore

package main

func main() {
	if len(os.Args) == 1 {
		src, err := os.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		err = slang.ParseBytes(src)
		if err != nil {
			panic(err)
		}
		return
	}
	for _, file := range os.Args[1:] {
		if err := slang.ParseFile(file); err != nil {
			panic(err)
		}
	}
}
