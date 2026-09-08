package pdfdump

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type Limits struct {
	InputBytes        int64
	DecompressedBytes int64
}

// InspectionLimitError reports a PDF inspection byte ceiling violation.
type InspectionLimitError struct {
	Stage  string
	Limit  int64
	Actual int64
}

func (e *InspectionLimitError) Error() string {
	return fmt.Sprintf("%s exceeds byte limit %d (actual=%d)", e.Stage, e.Limit, e.Actual)
}

var defaultLimits = Limits{InputBytes: 128 << 20, DecompressedBytes: 64 << 20}

// DumpPDF reads a PDF file and prints its structure with bounded parsing.
func DumpPDF(pdfPath string) error {
	return DumpPDFTo(pdfPath, os.Stdout, defaultLimits)
}

type errorWriter struct {
	w   io.Writer
	err error
}

func (w *errorWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

type boundedDumper struct {
	out    io.Writer
	limits Limits
}

// DumpPDFTo writes a bounded PDF inspection to out.
func DumpPDFTo(pdfPath string, out io.Writer, limits Limits) (returnErr error) {
	if out == nil {
		return fmt.Errorf("PDF dump output writer is required")
	}
	if limits.InputBytes <= 0 || limits.DecompressedBytes <= 0 {
		return fmt.Errorf("PDF dump limits must be positive")
	}
	checkedOut := &errorWriter{w: out}
	defer func() {
		if checkedOut.err != nil {
			returnErr = fmt.Errorf("write PDF dump: %w", checkedOut.err)
		}
	}()
	return (&boundedDumper{out: checkedOut, limits: limits}).dump(pdfPath)
}

func (d *boundedDumper) dump(pdfPath string) error {
	data, err := readBoundedPDF(pdfPath, d.limits.InputBytes)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.out, "PDF Stream Dump: %s\n", pdfPath)
	fmt.Fprintln(d.out, strings.Repeat("=", 80))
	matches := objRe.FindAllSubmatch(data, -1)
	for _, match := range matches {
		if err := d.dumpObject(match); err != nil {
			return err
		}
	}
	fmt.Fprintln(d.out, "\n"+strings.Repeat("=", 80))
	fmt.Fprintf(d.out, "Total objects processed: %d\n", len(matches))
	return nil
}

func readBoundedPDF(pdfPath string, limit int64) ([]byte, error) {
	file, err := os.Open(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, &InspectionLimitError{Stage: "PDF input", Limit: limit, Actual: int64(len(data))}
	}
	return data, nil
}

func (d *boundedDumper) dumpObject(match [][]byte) error {
	objNum, objBody := string(match[1]), match[2]
	streamMatch := streamRe.FindSubmatch(objBody)
	if streamMatch == nil {
		d.dumpObjectWithoutStream(objNum, objBody)
		return nil
	}
	return d.dumpStreamObject(objNum, objBody, streamMatch[1])
}

func (d *boundedDumper) dumpObjectWithoutStream(objNum string, objBody []byte) {
	dictBody := strings.TrimSpace(string(objBody))
	optional := isOptionalObject(dictBody)
	if len(dictBody) > 500 {
		dictBody = dictBody[:500] + "..."
	}
	if !isBinaryContent([]byte(dictBody)) && len(dictBody) < 2000 {
		fmt.Fprintf(d.out, "\n%s\n", objectHeader(objNum, optional))
		fmt.Fprintln(d.out, "  Kind: no stream")
		fmt.Fprintln(d.out, indentLines(formatDictionary(dictBody), "  "))
	}
}

func (d *boundedDumper) dumpStreamObject(objNum string, objBody, rawStream []byte) error {
	streamData := trimPDFStream(rawStream)
	dictPart := objBody[:bytes.Index(objBody, []byte("stream"))]
	dictStr := strings.TrimSpace(string(dictPart))
	isHint := isHintBinaryStream(dictStr)
	d.dumpStreamHeader(objNum, dictStr, isHint)
	decodedData, err := d.decodeStream(objNum, dictPart, streamData)
	if err != nil {
		return err
	}
	formatted := formatDecodedStream(decodedData, dictPart, dictStr, isHint)
	if formatted != "" {
		fmt.Fprint(d.out, indentLines(formatted, "    "))
	}
	return nil
}

func trimPDFStream(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte("\r\n"))
	data = bytes.TrimPrefix(data, []byte("\n"))
	data = bytes.TrimPrefix(data, []byte("\r"))
	return bytes.TrimRight(data, "\r\n \t")
}

func (d *boundedDumper) dumpStreamHeader(objNum, dictStr string, isHint bool) {
	fmt.Fprintf(d.out, "\n%s\n", objectHeader(objNum, isOptionalObject(dictStr)))
	if isHint {
		fmt.Fprintln(d.out, "  Kind: Hint stream")
	}
	fmt.Fprintln(d.out, "  Dictionary:")
	fmt.Fprint(d.out, indentLines(annotateDictStartWithObjectID(formatDictionary(dictStr), objNum), "    "))
}

func (d *boundedDumper) decodeStream(objNum string, dictPart, streamData []byte) ([]byte, error) {
	if !bytes.Contains(dictPart, []byte("FlateDecode")) {
		fmt.Fprintf(d.out, "  Stream (%d bytes, uncompressed):\n", len(streamData))
		return streamData, nil
	}
	decodedData, err := decompressFlate(streamData, d.limits.DecompressedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress object %s stream: %w", objNum, err)
	}
	fmt.Fprintf(d.out, "  Stream (decompressed %d -> %d bytes):\n", len(streamData), len(decodedData))
	return decodedData, nil
}

func formatDecodedStream(data, dictPart []byte, dictStr string, isHint bool) string {
	switch {
	case bytes.Contains(dictPart, []byte("/ObjStm")), bytes.Contains(dictPart, []byte("/Type/ObjStm")):
		return formatObjectStreamContent(data)
	case bytes.Contains(dictPart, []byte("/Type /XRef")), bytes.Contains(dictPart, []byte("/Type/XRef")):
		return formatXRefStream(data, dictStr)
	case isHint:
		return formatHintStream(data, dictStr)
	default:
		return formatStreamContent(data)
	}
}
