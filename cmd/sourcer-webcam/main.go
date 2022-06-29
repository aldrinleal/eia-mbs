package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aldrinleal/eia-mbs/plugininterface"
	"github.com/blackjack/webcam"
	"github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"os"
	"sort"
	"time"
)

const (
	V4L2_PIX_FMT_PJPG = 0x47504A50
	V4L2_PIX_FMT_YUYV = 0x56595559
)

var supportedFormats = map[webcam.PixelFormat]bool{
	V4L2_PIX_FMT_PJPG: true,
	V4L2_PIX_FMT_YUYV: true,
}

type FrameSizes []webcam.FrameSize

func (slice FrameSizes) Len() int {
	return len(slice)
}

//For sorting purposes
func (slice FrameSizes) Less(i, j int) bool {
	ls := slice[i].MaxWidth * slice[i].MaxHeight
	rs := slice[j].MaxWidth * slice[j].MaxHeight
	return ls < rs
}

//For sorting purposes
func (slice FrameSizes) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

type SourcerImpl struct {
	path string
	cam  *webcam.Webcam
	li   chan *bytes.Buffer
}

func NewSourcerImpl(path string) (*SourcerImpl, error) {
	cam, err := webcam.Open(path)

	if nil != err {
		return nil, err
	}

	// select pixel format
	format_desc := cam.GetSupportedFormats()

	log.Infof("Available formats:")
	for _, s := range format_desc {
		log.Infof(s)
	}

	// TODO: Read as Parameters
	fmtstr := ""
	szstr := ""

	var format webcam.PixelFormat
FMT:
	for f, s := range format_desc {
		if fmtstr == "" {
			if supportedFormats[f] {
				format = f
				break FMT
			}

		} else if fmtstr == s {
			if !supportedFormats[f] {
				return nil, fmt.Errorf("format '%s' is not supported, exiting", format_desc[f])
			}
			format = f
			break
		}
	}

	if format == 0 {
		return nil, errors.New("No format found, exiting")
	}

	// select frame size
	frames := FrameSizes(cam.GetSupportedFrameSizes(format))
	sort.Sort(frames)

	log.Infof("Supported frame sizes for format %s", format_desc[format])
	for _, f := range frames {
		log.Infof(f.GetString())
	}

	var size *webcam.FrameSize
	if szstr == "" {
		size = &frames[len(frames)-1]
	} else {
		for _, f := range frames {
			if szstr == f.GetString() {
				size = &f
			}
		}
	}

	if size == nil {
		return nil, errors.New("No matching frame size, exiting")
	}

	f, w, h, err := cam.SetImageFormat(format, uint32(size.MaxWidth), uint32(size.MaxHeight))

	if nil != err {
		return nil, err
	}

	var (
		li   chan *bytes.Buffer = make(chan *bytes.Buffer)
		fi   chan []byte        = make(chan []byte)
		back chan struct{}      = make(chan struct{})
	)

	go encodeToImage(cam, back, fi, li, w, h, f)

	err = cam.StartStreaming()

	if nil != err {
		return nil, err
	}

	timeout := uint32(1)

	go func() {
		for {
			err = cam.WaitForFrame(timeout)
			if err != nil {
				log.Println(err)
				return
			}

			switch err.(type) {
			case nil:
			case *webcam.Timeout:
				log.Fatalf("Timed Out: %s", err)
				continue
			default:
				log.Fatalf("Oops: %s", err)
				return
			}

			frame, err := cam.ReadFrame()
			if err != nil {
				log.Fatalf("Oops: %s", err)
				return
			}

			if len(frame) != 0 {

				// print framerate info every 10 seconds
				//fr++
				//if *fps {
				//	if d := time.Since(start); d > time.Second*10 {
				//		fmt.Println(float64(fr)/(float64(d)/float64(time.Second)), "fps")
				//		start = time.Now()
				//		fr = 0
				//	}
				//}

				select {
				case fi <- frame:
					<-back
				default:
				}
			}
		}
	}()

	return &SourcerImpl{
		path: path,
		cam:  cam,
		li:   li,
	}, nil
}

func encodeToImage(wc *webcam.Webcam, back chan struct{}, fi chan []byte, li chan *bytes.Buffer, w, h uint32, format webcam.PixelFormat) {
	var (
		frame []byte
		img   image.Image
	)
	for {
		bframe := <-fi
		// copy frame
		if len(frame) < len(bframe) {
			frame = make([]byte, len(bframe))
		}
		copy(frame, bframe)
		back <- struct{}{}

		switch format {
		case V4L2_PIX_FMT_YUYV:
			yuyv := image.NewYCbCr(image.Rect(0, 0, int(w), int(h)), image.YCbCrSubsampleRatio422)
			for i := range yuyv.Cb {
				ii := i * 4
				yuyv.Y[i*2] = frame[ii]
				yuyv.Y[i*2+1] = frame[ii+2]
				yuyv.Cb[i] = frame[ii+1]
				yuyv.Cr[i] = frame[ii+3]

			}
			img = yuyv
		default:
			log.Fatal("invalid format ?")
		}
		//convert to jpeg
		buf := &bytes.Buffer{}
		if err := jpeg.Encode(buf, img, nil); err != nil {
			log.Fatal(err)
			return
		}

		const N = 50
		// broadcast image up to N ready clients
		nn := 0
	FOR:
		for ; nn < N; nn++ {
			select {
			case li <- buf:
			default:
				break FOR
			}
		}
		if nn == 0 {
			li <- buf
		}

	}
}

func (i *SourcerImpl) GetImage() plugininterface.SourcerReply {
	response := plugininterface.SourcerReply{}

	<-i.li

	img := <-i.li

	response.Data = img.Bytes()
	response.Metadata = make(map[string]string)
	response.Metadata["content-type"] = "image/jpeg"
	response.Metadata["name"] = "image-" + fmt.Sprintf("%08X.jpg", time.Now().Unix())

	return response
}

func main() {
	sourcerImpl, err := NewSourcerImpl(os.Args[1])

	if nil != err {
		log.Fatalf("creating sourcer: %s", err)
	}

	pluginMap := map[string]plugin.Plugin{
		"sourcer": &plugininterface.SourcerPlugin{Impl: sourcerImpl},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugininterface.HandshakeConfig,
		Plugins:         pluginMap,
	})

}
