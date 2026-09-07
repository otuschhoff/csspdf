module github.com/otuschhoff/csspdf

go 1.25.13

require (
	github.com/andybalholm/cascadia v1.3.3
	github.com/aymerick/douceur v0.2.0
	github.com/goodsign/monday v1.0.2
	github.com/otuschhoff/gofpdf v0.0.0-20250501183920-cb47a268fb6d
	golang.org/x/image v0.43.0
	golang.org/x/net v0.56.0
)

require github.com/gorilla/css v1.0.1 // indirect

replace github.com/otuschhoff/gofpdf => ./third_party/gofpdf
