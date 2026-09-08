module example.com/csspdf-external-consumer

go 1.25.13

require github.com/otuschhoff/csspdf v0.0.0

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/goodsign/monday v1.0.2 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/otuschhoff/gofpdf v0.0.0-20250501183920-cb47a268fb6d // indirect
	golang.org/x/net v0.56.0 // indirect
)

replace github.com/otuschhoff/csspdf => ../../..

replace github.com/otuschhoff/gofpdf => ../../../third_party/gofpdf
