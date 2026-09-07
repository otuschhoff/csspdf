module github.com/otuschhoff/csspdf

go 1.25.13

require (
	github.com/andybalholm/cascadia v1.3.3
	github.com/aymerick/douceur v0.2.0
	github.com/goodsign/monday v1.0.2
	github.com/otuschhoff/gofpdf v0.0.0-20250501183920-cb47a268fb6d
	github.com/pdfcpu/pdfcpu v0.13.0
	golang.org/x/image v0.43.0
	golang.org/x/net v0.56.0
)

require (
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hhrutter/lzw v1.0.0 // indirect
	github.com/hhrutter/pkcs7 v0.2.2 // indirect
	github.com/hhrutter/tiff v1.0.3 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/otuschhoff/gofpdf => ./third_party/gofpdf
