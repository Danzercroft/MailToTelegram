// Package provides functions generating PNG from diferent types
package png

import (
	"log"
	"strings"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"gopkg.in/gographics/imagick.v2/imagick"
)

// Convert HTML to PNG
func HtmlToPng(html string) []byte {
	// Create new PDF generator
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		log.Fatal(err)
	}

	// Set the conversion options
	pdfg.Dpi.Set(300)
	pdfg.Orientation.Set(wkhtmltopdf.OrientationLandscape)
	pdfg.Grayscale.Set(true)
	pdfg.ImageDpi.Set(300)

	// Convert the HTML to PDF
	pdfg.AddPage(wkhtmltopdf.NewPageReader(strings.NewReader(html)))
	err = pdfg.Create()
	if err != nil {
		panic(err)
	}

	imagick.Initialize()
	defer imagick.Terminate()
	mw := imagick.NewMagickWand()
	defer mw.Destroy()
	mw.SetImageUnits(imagick.RESOLUTION_PIXELS_PER_INCH)
	mw.SetResolution(300, 300)
	err = mw.ReadImageBlob(pdfg.Bytes())
	if err != nil {
		panic(err)
	}
	mw.SetIteratorIndex(0) // This being the page offset
	mw.SetImageFormat("png")
	// Append all the pages into a single wand
	mw = mw.AppendImages(true)
	mw.SetImageUnits(imagick.RESOLUTION_PIXELS_PER_INCH)
	mw.SetImageResolution(300, 300)

	// Return a new PNG image
	return mw.GetImageBlob()
}
