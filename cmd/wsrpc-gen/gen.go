package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
)

var (
	inputFile     = flag.String("input", "", "Go file containing the interface")
	interfaceName = flag.String("interface", "", "Name of the interface to generate client and server for")
	outputFile    = flag.String("output", "wsrpc_gen.go", "Output file")
)

func main() {
	flag.Parse()

	if *inputFile == "" || *interfaceName == "" {
		log.Fatal("input and interface flags are required")
	}

	src, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("failed to read input file: %v", err)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, *inputFile, src, parser.AllErrors)
	if err != nil {
		log.Fatalf("failed to parse input: %v", err)
	}

	packageName := node.Name.Name
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "package %s\n\n", packageName)
	fmt.Fprintf(&buf, "import (\n\t\"context\"\n\t\"fmt\"\n\t\"github.com/yingshulu/wsrpc/rpc\"\n)\n\n")

	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != *interfaceName {
			return true
		}
		iface, ok := typeSpec.Type.(*ast.InterfaceType)
		if !ok {
			log.Fatalf("%s is not an interface", *interfaceName)
		}

		clientName := *interfaceName + "Client"
		fmt.Fprintf(&buf, "type %s struct {\n\th rpc.ServiceHolder\n\tpeer string\n}\n\n", clientName)
		fmt.Fprintf(&buf, "func New%s(h rpc.ServiceHolder, peer string) *%s {\n\treturn &%s{h: h, peer: peer}\n}\n\n", clientName, clientName, clientName)

		for _, method := range iface.Methods.List {
			if len(method.Names) == 0 {
				continue
			}
			name := method.Names[0].Name
			sig, ok := method.Type.(*ast.FuncType)
			if !ok || !isExported(name) || sig.Params.NumFields() < 2 || sig.Results.NumFields() < 2 {
				continue
			}
			req := exprString(sig.Params.List[1].Type)
			res := exprString(sig.Results.List[0].Type)

			generateProxyClientMethod(&buf, packageName, *interfaceName, clientName, name, req, res)
		}

		fmt.Fprintf(&buf, "func Register%sService(service %s, options ...rpc.Option) {\n", *interfaceName, *interfaceName)
		fmt.Fprintf(&buf, "\trpc.RegisterService(\"%s.%s\", service, options...)\n", packageName, *interfaceName)
		fmt.Fprintf(&buf, "}\n")
		return false
	})

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("failed to format generated code: %v", err)
	}
	if err := os.WriteFile(*outputFile, formatted, 0644); err != nil {
		log.Fatalf("failed to write output file: %v", err)
	}
	fmt.Printf("Generated client/server written to %s\n", *outputFile)
}

func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.ArrayType:
		return "[]" + exprString(e.Elt)
	default:
		return fmt.Sprintf("%#v", expr)
	}
}

func generateProxyClientMethod(buf *bytes.Buffer, packageName, interfaceName, clientName, methodName, req, res string) {
	fmt.Fprintf(buf, "func (c *%s) %s(ctx context.Context, req %s, options ...rpc.Option) (%s, error) {\n", clientName, methodName, req, res)
	fmt.Fprintf(buf, "\tvar res %s\n", res)
	fmt.Fprintf(buf, "\tconn := c.h.GetConnByPeer(c.peer)\n\tif conn == nil {\n\t\treturn res, fmt.Errorf(\"c.peer connection not found\")\n\t}\n")
	fmt.Fprintf(buf, "\tproxy := conn.GetProxy(\"%s.%s.%s\")\n", packageName, interfaceName, methodName)
	fmt.Fprintf(buf, "\terr := proxy.Call(ctx, req, res, options...)\n")
	fmt.Fprintf(buf, "\treturn res, err\n")
	fmt.Fprintf(buf, "}\n\n")
}

func isExported(name string) bool {
	return name[0] >= 'A' && name[0] <= 'Z'
}
