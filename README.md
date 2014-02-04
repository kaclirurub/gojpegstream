gojpegstream prepares a list of JPEG files for a video encoder (like x264)

By Brandon Thomson <bt@brandonthomson.com> [www.brandonthomson.com](http://www.brandonthomson.com)
(2-clause BSD license)

--

gojpegstream is for the case where you have a folder full of JPEG files that
you want to encode as the frames of a movie. It will decode the JPEGs and write
them to stdout, to be piped into your favorite video encoder.

gojpegstream accepts a newline-separated list of JPEGs on its standard input.

Go isn't the fastest way to decode JPEGs, but gojpegstream should be fast
enough that your encoder is not idling while waiting for new frames. On an
older 8-logical core CPU, gojpegstream was able to serve about 140fps with 5
threads and 640x480 JPEG files. That was enough to max out x264.

By default, gojpegstream will use `runtime.NumCPU()` threads for decoding. This
can be overridden with `-numthreads`.

gojpegstream has been tested with x264 but should work with other video
encoders that accept unpacked YUV 4:2:2 input.

Example script for using gojpegstream with x264:

```shell
mkfifo /tmp/foo
cd /path/to/jpegs
ls . | gojpegstream > /tmp/foo &
x264 --input-csp i422 --output-csp i422 -o /tmp/foo.mkv /tmp/foo
```

Note that i422 is a non-standard H.264 format. Depending on how you compiled
your x264, it may be able to convert the output to the more common i420 format.
mplayer can play back i422 just fine but other players may not be able to.


gojpegstream accepts a list of JPEG files on stdin rather than on the command
line because some platforms only allow a limited number of arguments to a
program. On these platforms, scripts written to use a list of JPEG files as
arguments would be subject to failure if the number of JPEGs got too large.
