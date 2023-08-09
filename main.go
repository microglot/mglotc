package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"

	"gopkg.microglot.org/compiler.go/internal/compiler"
	"gopkg.microglot.org/compiler.go/internal/fs"
	"gopkg.microglot.org/compiler.go/internal/idl"
)

type opts struct {
	Roots  []string
	Output string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := &opts{}
	flags := pflag.NewFlagSet("microglotc", pflag.PanicOnError)
	flags.StringSliceVar(&op.Roots, "root", []string{"."}, "Root search paths for imports.")
	flags.StringVar(&op.Output, "output", ".", "Output directory or - for STDOUT.")
	_ = flags.Parse(os.Args[1:])
	targets := flags.Args()

	output, absErr := filepath.Abs(op.Output)
	if absErr != nil {
		panic(absErr)
	}

	f, err := compiler.NewDefaultFS(os.LookupEnv)
	if err != nil {
		panic(err)
	}

	mf := make(fs.FileSystemMulti, 0, len(op.Roots)+1)
	for _, root := range op.Roots {
		absRoot, errAbs := filepath.Abs(root)
		if errAbs != nil {
			panic(errAbs.Error())
		}
		rf, err := fs.NewFileSystemLocal(absRoot)
		if err != nil {
			panic(err.Error())
		}
		mf = append(mf, rf)
	}
	mf = append(mf, f)

	c, err := compiler.New(
		compiler.OptionWithLookupEnv(os.LookupEnv),
		compiler.OptionWithFS(mf),
	)
	if err != nil {
		panic(err)
	}

	out, err := c.Compile(ctx, &idl.CompileRequest{
		Files: targets,
	})
	if err != nil {
		var me compiler.MultiException
		if errors.As(err, &me) {
			for _, err := range me {
				fmt.Fprintln(os.Stderr, err.Error())
			}
			os.Exit(1)
		}
		panic(err)
	}
	fmt.Println(out.Image)
	fmt.Println(output)
}
