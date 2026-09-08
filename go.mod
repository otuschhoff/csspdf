module github.com/otuschhoff/csspdf

go 1.26.6

require (
	github.com/andybalholm/cascadia v1.3.5
	github.com/aymerick/douceur v0.2.0
	github.com/goodsign/monday v1.0.2
	github.com/otuschhoff/gofpdf v0.0.0-20250501183920-cb47a268fb6d
	github.com/pdfcpu/pdfcpu v0.15.0
	golang.org/x/image v0.45.0
	golang.org/x/net v0.58.0
	golang.org/x/tools v0.49.0
)

require (
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hhrutter/tiff v1.0.6 // indirect
	github.com/mattn/go-runewidth v0.0.29 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/mod v0.39.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/otuschhoff/gofpdf => ./third_party/gofpdf
