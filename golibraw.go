// go:build (darwin && cgo) || linux

package golibraw

// #cgo LDFLAGS: -lraw
// #include <libraw/libraw.h>
import "C"

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"unsafe"

	"github.com/lmittmann/ppm"
)

type Camera struct {
	Make     string
	Model    string
	Software string
	Colors   uint
}

type Lens struct {
	Make           string
	Model          string
	Serial         string
	MinFocal       float64
	MaxFocal       float64
	MaxAp4MinFocal float64
	MaxAp4MaxFocal float64
}

type Metadata struct {
	Timestamp int64
	Width     int
	Height    int
	DataSize  int64
	Camera    Camera
	Lens      Lens
	ISO       int
	Aperture  float64
	Shutter   float64
}

type rawImg struct {
	Height   int
	Width    int
	Bits     uint
	DataSize int
	Data     []byte
}

func (r rawImg) fullBytes() []byte {
	header := fmt.Sprintf("P6\n%d %d\n%d\n", r.Width, r.Height, (1<<r.Bits)-1)
	return append([]byte(header), r.Data...)
}

func goResult(result C.int) error {
	if int(result) == 0 {
		return nil
	}
	p := C.libraw_strerror(result)
	return fmt.Errorf("libraw error: %v", C.GoString(p))
}

func lrInit() *C.libraw_data_t {
	librawProcessor := C.libraw_init(0)
	return librawProcessor
}

// Reads a RAW image file from file system and exports the embedded thumbnail image - if it exists - to the path defined by exportPath parameter.
// This method is significantly faster than importing the RAW image file.
func ExtractThumbnail(inputPath string, exportPath string) error {
	if _, err := os.Stat(exportPath); err == nil {
		return fmt.Errorf("output file [%v] already exists", exportPath)
	}

	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file [%v] does not exist", exportPath)
	}

	librawProcessor := lrInit()
	defer C.libraw_recycle(librawProcessor)

	if err := goResult(C.libraw_open_file(librawProcessor, C.CString(inputPath))); err != nil {
		return fmt.Errorf("failed to open input file [%v]", inputPath)
	}

	if err := goResult(C.libraw_unpack_thumb(librawProcessor)); err != nil {
		return fmt.Errorf("unpacking thumbnail from RAW failed with [%v]", err)
	}

	if err := goResult(C.libraw_dcraw_thumb_writer(librawProcessor, C.CString(exportPath))); err != nil {
		return fmt.Errorf("writing thumbnail failed with [%v]", err)
	}

	return nil
}

// Reads a RAW image file from file system and exports collected metadata.
// This method is significantly faster than importing the RAW image file.
func ExtractMetadata(path string) (Metadata, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("input file does not exist [%v]", path)
	}

	librawProcessor := lrInit()
	defer C.libraw_recycle(librawProcessor)

	if err := goResult(C.libraw_open_file(librawProcessor, C.CString(path))); err != nil {
		return Metadata{}, fmt.Errorf("failed to open input file [%v]", path)
	}

	iparam := C.libraw_get_iparams(librawProcessor)
	lensinfo := C.libraw_get_lensinfo(librawProcessor)
	other := C.libraw_get_imgother(librawProcessor)
	width := int(C.libraw_get_raw_width(librawProcessor))
	height := int(C.libraw_get_raw_height(librawProcessor))

	metadata := Metadata{
		Timestamp: int64(other.timestamp),
		Width:     int(width),
		Height:    int(height),
		DataSize:  stat.Size(),
		Camera: Camera{
			Make:     C.GoString(&iparam.normalized_make[0]),
			Model:    C.GoString(&iparam.normalized_model[0]),
			Software: C.GoString(&iparam.software[0]),
			Colors:   uint(iparam.colors),
		},
		Lens: Lens{
			Make:           C.GoString(&lensinfo.LensMake[0]),
			Model:          C.GoString(&lensinfo.Lens[0]),
			Serial:         C.GoString(&lensinfo.LensSerial[0]),
			MinFocal:       float64(lensinfo.MinFocal),
			MaxFocal:       float64(lensinfo.MaxFocal),
			MaxAp4MinFocal: float64(lensinfo.MaxAp4MinFocal),
			MaxAp4MaxFocal: float64(lensinfo.MaxAp4MaxFocal),
		},
		ISO:      int(other.iso_speed),
		Aperture: float64(other.aperture),
		Shutter:  float64(other.shutter),
	}
	return metadata, nil
}

// Reads a RAW image file from file system and converts it to standard image.Image
func ImportRaw(path string) (image.Image, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("input file [%v] does not exist", path)
	}

	librawProcessor := lrInit()
	defer C.libraw_recycle(librawProcessor)

	err := goResult(C.libraw_open_file(librawProcessor, C.CString(path)))
	if err != nil {
		return nil, fmt.Errorf("failed to open file [%v]", path)
	}

	err = goResult(C.libraw_unpack(librawProcessor))
	if err != nil {
		return nil, fmt.Errorf("failed to unpack file [%v]", path)
	}

	err = goResult(C.libraw_dcraw_process(librawProcessor))
	if err != nil {
		return nil, fmt.Errorf("failed to import file [%v]", path)
	}

	var result C.int

	img := C.libraw_dcraw_make_mem_image(librawProcessor, &result)
	defer C.libraw_dcraw_clear_mem(img)

	if goResult(result) != nil {
		return nil, fmt.Errorf("failed to import file [%v]", path)
	}
	dataBytes := make([]uint8, int(img.data_size))
	start := unsafe.Pointer(&img.data)
	size := unsafe.Sizeof(uint8(0))
	for i := 0; i < int(img.data_size); i++ {
		item := *(*uint8)(unsafe.Pointer(uintptr(start) + size*uintptr(i)))
		dataBytes[i] = item
	}

	rawImage := rawImg{
		Height:   int(img.height),
		Width:    int(img.width),
		DataSize: int(img.data_size),
		Bits:     uint(img.bits),
		Data:     dataBytes,
	}

	fullbytes := rawImage.fullBytes()
	return ppm.Decode(bytes.NewReader(fullbytes))
}

// Reads a RAW image file from file system and exports it to PPM format
func ExportPPM(inputPath string, exportPath string) error {
	if _, err := os.Stat(exportPath); err == nil {
		return fmt.Errorf("output file [%v] already exists", exportPath)
	}

	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("input file [%v] does not exist", exportPath)
	}

	librawProcessor := lrInit()
	defer C.libraw_recycle(librawProcessor)

	err := goResult(C.libraw_open_file(librawProcessor, C.CString(inputPath)))
	if err != nil {
		return fmt.Errorf("failed to open file [%v]", inputPath)
	}

	err = goResult(C.libraw_unpack(librawProcessor))
	if err != nil {
		return fmt.Errorf("failed to unpack file [%v]", inputPath)
	}

	err = goResult(C.libraw_dcraw_process(librawProcessor))
	if err != nil {
		return fmt.Errorf("failed to import file [%v]", inputPath)
	}

	if err = goResult(C.libraw_dcraw_ppm_tiff_writer(librawProcessor, C.CString(exportPath))); err != nil {
		return fmt.Errorf("failed to export file to [%v]", exportPath)
	}
	return nil
}

func lrClose(iprc *C.libraw_data_t) {
	C.libraw_close(iprc)
}
