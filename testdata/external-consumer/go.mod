module example.com/csspdf-external-consumer

go 1.26.6

require github.com/otuschhoff/csspdf v0.0.0

require (
	github.com/andybalholm/cascadia v1.3.5 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/goodsign/monday v1.0.2 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/otuschhoff/gofpdf v0.0.0-20260909105358-90766bef84b1 // indirect
	golang.org/x/net v0.58.0 // indirect
)

replace github.com/otuschhoff/csspdf => ../..
