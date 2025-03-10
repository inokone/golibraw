# golibraw

Simple Go wrapper for [libraw](https://www.libraw.org/)

Forked from [enricod/golibraw](https://github.com/enricod/golibraw).

## Prerequisites

``` sh
brew install libraw                       # on OSX
sudo apt-get install libraw-dev           # on Ubuntu

go install github.com/inokone/golibraw@latest
```

## Usage example

``` go
import (
    "bytes"
    "fmt"
    "image/jpeg"
    raw "github.com/inokone/golibraw"
)

func main() {
    path := "/Downloads/example.cr2"

    // Get metadata from the RAW image on Camera, Lens and Settings - fast operation
    metadata, err := raw.ExtractMetadata(path)
    if err != nil {
        fmt.Println("Metadata export error: ", err)
    }
    fmt.Println("Metadata: ", metadata)

    // Get the embedded thumbnail from RAW if exists - fast operation
    err = raw.ExtractThumbnail(path, "/Downloads/out.jpeg")
    if err != nil {
        fmt.Println("Error while extracting embedded thumbnail, maybe it is not present: ", err)
    }

    // Import the RAW file as an image.Image - slow operation
    image, err := raw.ImportRaw(path)
    if err != nil {
        fmt.Println("RAW import error: ", err)
    }

    // Export the image as JPEG 
    buf := new(bytes.Buffer)
    err = jpeg.Encode(buf, image, nil)
    if err != nil {
        fmt.Println("Error while exporting image: ", err)
    }
}
```
