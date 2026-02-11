package pdflib

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"path/filepath"
)

var referenceFirstPage = `TEST FILE 
 
Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam 
nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam 
erat, sed diam voluptua. At vero eos et accusam et 
TEST 
SUBTITLE`

var referenceFirstPageWithAddLine = `TEST FILE 
 
Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam 
nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam 
erat, sed diam voluptua. At vero eos et accusam et
 
TEST 
SUBTITLE`

//
// this pdf has an object within stream which is handled different!
// the original implementation calculated the stream but didn't returned the object at resolve
//
// @todo: there is an empty line added, still don't know where
//
func Test_ReadPdf_v17_linarized_xrefStream(t *testing.T) {

	testFile := "./testdata/story_Word2019-2312-1601712620132-32_Print-Adobe__pdf15_linarized_xrefStream.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect numPage .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPageWithAddLine != content {
		t.Error("Asser: content different from reference:")
		t.Error(content)
	}
}
func Test_ReadPdf_v17_linarized_xref(t *testing.T) {

	testFile := "./testdata/story_avepdf-com__pdf17_linarized_xref.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect numPage .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPage != content {
		t.Error("Asser: content different from reference:")
		t.Error(content)
	}
}
//
// this pdf has an array of refs at /Contents
//	standard:
// page = {<</Contents 4 0 R /Group <</CS /DeviceRGB /S /Transparency /Type /Group>> /MediaBox [0 0 612 792] /Parent 2 0 R /Resources <</ExtGState <</GS7 7 0 R /GS8 8 0 R>> /Font <</F1 5 0 R /F2 9 0 R /F3 11 0 R>> /ProcSet [/PDF /Text /ImageB /ImageC /ImageI]>> /StructParents 0 /Type /Page>>}
//	deviation:
// page = {<</Contents [20 0 R] /CropBox [0 0 595.32001 841.92004] /MediaBox [0 0 595.32001 841.92004] /Parent 2 0 R /Resources 21 0 R /Rotate 0 /Type /Page>>}
//
func Test_ReadPdf_v17_trailer_arrayAtPageContents(t *testing.T) {

	testFile := "./testdata/story_Word2019-2312-1712620132_Print-Microsoft__pdf17_trailer_array-at-page-contents.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect numPage .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPage != content {
		t.Error("Asser: content different from reference:")
		t.Error(content)
	}
}
func Test_ReadPdf_v17_StandardPDFA_trailer(t *testing.T) {

	testFile := "./testdata/story_Word2019-2312-1712620132_SaveAs-Standard-PDFA__pdf17_trailer.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect numPage .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPage != content {
		t.Error("Asser: content different from reference:")
		t.Error(content)
	}
}
func Test_ReadPdf_v17_MinSizePDFA_trailer(t *testing.T) {

	testFile := "./testdata/story_Word2019-2312-1712620132_SaveAs-MinSize-PDFA__pdf17_trailer.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect if totalPages != 5 { .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPage != content {
		t.Error("Asser: content different from reference")
		t.Error(content)
	}
}
func Test_ReadPdf_v17_StandardNoPDFA_2trailer(t *testing.T) {

	testFile := "./testdata/story_Word2019-2312-1712620132_SaveAs-Standard-NoPDFA__pdf17_2trailer.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect totalPages .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPage != content {
		t.Error("Asser: content different from reference")
		t.Error(content)
	}
}
func Test_ReadPdf_v17_MinSizeNoPDFA_2trailer(t *testing.T) {

	testFile := "./testdata/story_Word2019-2312-1712620132_SaveAs-MinSize-NoPDFA__pdf17_2trailer.pdf"
	totalPages, content := readPdfAndGetFirstPageAsText(testFile)
	if totalPages != 5 {
		t.Error("Asser: incorrect totalPages .. want=5 <> got " + strconv.Itoa(totalPages))
	}
	if referenceFirstPage != content {
		t.Error("Asser: content different from reference")
		t.Error(content)
	}
}
//
// read pdf and return content of first page for quick check
//
func readPdfAndGetFirstPageAsText(fileName string) (totalPages int, content string) {
	fmt.Println("read file = " + fileName)
	
	f, err := Open(fileName)
	if err != nil {
		return 0, err.Error()
	}

	totalPages = f.NumPage()
	if totalPages == 0 {
		return totalPages, content
	} else {
	
		var buf bytes.Buffer
		p := f.Page(1)
		texts := p.Content().Text
		var lastY = 0.0
		line := ""

		for _, text := range texts {
			if lastY != text.Y {
				if lastY > 0 {
					buf.WriteString(line + "\n")
					line = text.S
				} else {
					line += text.S
				}
			} else {
				line += text.S
			}

			lastY = text.Y
		}
		buf.WriteString(line)
		content = strings.TrimSpace(buf.String())
	}
	
	return totalPages, content
}
//
// process all pdfs within ./testdata/*.pdf and write content to *.txt
//
func Test_WalkDirectory_ReadPdfs(t *testing.T) {
	
	// get files
	var startPath string = "./testdata"
	files, err := walkDir(startPath, ".pdf")
	if err != nil {
        t.Error("Assert: " + err.Error())
    }
	
	// read files
	for i:=0; i<len(files); i++ {
		
		testFile := files[i]
		if !strings.HasSuffix(testFile, ".pdf") {
			continue
		}
		
fmt.Println(". open testFile = ", testFile)
		f, err := Open(testFile)
		if err != nil {
			t.Error(err)
		}

		totalPage := f.NumPage()
fmt.Println(". totalPage = ", totalPage)
		
		var buf bytes.Buffer

		for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		
			p := f.Page(pageIndex)
			if p.V.IsNull() {
				continue
			}
			
			texts := p.Content().Text
			var lastY = 0.0
			line := ""

			for _, text := range texts {
				if lastY != text.Y {
					if lastY > 0 {
						buf.WriteString(line + "\n")
						line = text.S
					} else {
						line += text.S
					}
				} else {
					line += text.S
				}

				lastY = text.Y
			}
			buf.WriteString(line)
		}
		
		//
		//fmt.Println(buf.String())
		
		//
		// write bytes buffer to txt-file
		writeToFileName := strings.Replace(testFile, ".pdf", ".txt", -1)
		fmt.Println(".. writeToFileName = ", writeToFileName)
		
		fw, err := os.Create(writeToFileName)
		if err != nil {
			t.Error(err)
		}
		_, err = fw.WriteString(buf.String())
		if err != nil {
			t.Error(err)
		}
		
		fw.Close()
	}
}
//
// walk indicated directory and 
// return all file.names with indicated suffix
//
func walkDir(root, fileSuffix string) ([]string, error) {
    var files []string
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if !info.IsDir() && strings.HasSuffix(path, fileSuffix) {
            files = append(files, path)
        }
        return nil
    })
    return files, err
}