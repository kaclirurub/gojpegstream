// App gojpegstream prepares a list of jpeg files for a video encoder (like x264).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os"
	"runtime"
	"time"
)

const (
	numOpenFilesToBuffer = 200 // doesn't actually do readahead; just opens them
	decodedImgBufferSize = 0   // thought this might be useful, but it seems not
)

var (
	debug      = flag.Bool("debug", false, "enable debugging output")
	nowrite    = flag.Bool("nowrite", false, "do not write data to stdout (for benchmarking)")
	nodecode   = flag.Bool("nodecode", false, "do not decode jpeg data (for benchmarking)")
	numThreads = flag.Int("numthreads", 0, "number of threads [0 = runtime.NumCPU()]")
	chdir      = flag.String("cd", "", "cd to this directory before starting")
)

// used to have a multiThreadReader, but it wasn't needed, not even over NFS
func singleThreadReader(ch chan *os.File, names []string) {
	go func() {
		for _, v := range names {
			f, err := os.Open(v)
			if err != nil {
				log.Fatal(err)
			}
			ch <- f
		}
		close(ch)
	}()
}

// much easier to understand than multiThreadDecoder
func singleThreadDecoder(ch chan *os.File, chw chan image.Image, totalSize *int64) {
	for f := range ch {
		stat, err := f.Stat()
		if err != nil {
			log.Fatal(err)
		}
		*totalSize += stat.Size()

		if !*nodecode {
			img, err := jpeg.Decode(f)
			if err != nil {
				log.Fatal(err)
			}
			chw <- img
		}

		f.Close()
	}
	close(chw)
}

func multiThreadDecoder(ch chan *os.File, chw chan image.Image, totalSize *int64) {
	var needsClose bool
	for {
		var files []*os.File

		for i := 0; i < *numThreads; i++ {
			f, ok := <-ch
			if !ok {
				// we're out of jpeg files. finish up the remaining frames
				// and then we'll exit at the end of this loop
				needsClose = true
				break
			}
			files = append(files, f)

			stat, err := f.Stat()
			if err != nil {
				log.Fatal(err)
			}
			*totalSize += stat.Size()
		}

		var chs []chan image.Image

		if !*nodecode {
			for i, f := range files {
				chs = append(chs, make(chan image.Image, 1))
				go func(i int, f *os.File) {
					img, err := jpeg.Decode(f)
					if err != nil {
						log.Fatal(err)
					}
					chs[i] <- img
				}(i, f)
			}

			for _, ch := range chs {
				chw <- <-ch
			}
		}

		for _, f := range files {
			f.Close()
		}

		if needsClose {
			close(chw)
			return
		}
	}
}

func main() {
	if *numThreads == 0 {
		*numThreads = runtime.NumCPU()
	}
	log.Printf("using %d threads", *numThreads)

	if *chdir != "" {
		if err := os.Chdir(*chdir); err != nil {
			log.Fatal(err)
		}
	}

	runtime.GOMAXPROCS(*numThreads)

	names := readStdin()

	// note well: image.Image is an interface and the backing store is a pointer.
	// so, there's no need to use a pointer to image.Image
	chw := make(chan image.Image, decodedImgBufferSize)
	ch := make(chan *os.File, numOpenFilesToBuffer)

	var totalSize int64

	start := time.Now()
	singleThreadReader(ch, names)
	if *numThreads == 1 {
		log.Print("using singleThreadDecoder")
		go singleThreadDecoder(ch, chw, &totalSize)
	} else {
		log.Print("using multiThreadDecoder")
		go multiThreadDecoder(ch, chw, &totalSize)
	}

	for img := range chw {
		if !*nowrite {
			os.Stdout.Write(img.(*image.YCbCr).Y)
			os.Stdout.Write(img.(*image.YCbCr).Cb)
			os.Stdout.Write(img.(*image.YCbCr).Cr)
		} else {
			fmt.Fprint(os.Stderr, "w")
		}
	}

	stop := time.Now()

	log.Printf("Total time to read %d frames: %0.1f secs", len(names), stop.Sub(start).Seconds())
	log.Printf("Total size of JPEG data: %0.2f MB", float64(totalSize)/1024/1024)
}

func readStdin() (lines []string) {
	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal(err)
		}
		lines = append(lines, line[:len(line)-1])
	}
}

func init() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	switch len(args) {
	case 0:
		return
	default:
		log.Print("Too many arguments.")
		exitUsage()
	}
}

func init() {
	log.SetPrefix(os.Args[0] + ":")
	log.SetFlags(log.Lshortfile)
}

func usage() {
	fmt.Fprintf(os.Stderr, "%s - %s\n\n", os.Args[0], "prepare a folder of JPEG files for x264")
	fmt.Fprintf(os.Stderr, "USAGE:\n  %s [options] /path/to/folder/of/jpegs\n", os.Args[0])
	flag.PrintDefaults()
}

func exitUsage() {
	usage()
	os.Exit(1)
}
