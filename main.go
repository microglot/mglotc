package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"gopkg.microglot.org/compiler.go/internal/compiler"
	"gopkg.microglot.org/compiler.go/internal/fs"
	"gopkg.microglot.org/compiler.go/internal/idl"
	"gopkg.microglot.org/compiler.go/internal/mgdl_gen_go"
	"gopkg.microglot.org/compiler.go/internal/target"
)

type opts struct {
	Roots            []string
	Output           string
	DumpTokens       bool
	DumpTree         bool
	DescriptorSetOut string
	ProtobufPlugins  []string
	Plugins          []string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := &opts{}
	flags := pflag.NewFlagSet("microglotc", pflag.PanicOnError)
	flags.StringSliceVar(&op.Roots, "root", []string{"."}, "Root search paths for imports.")
	flags.StringVar(&op.Output, "output", ".", "Output directory or - for STDOUT.")
	flags.BoolVar(&op.DumpTokens, "dump-tokens", false, "Output the token stream as it is processed")
	flags.BoolVar(&op.DumpTree, "dump-tree", false, "Output the parse tree after parsing")
	flags.StringVar(&op.DescriptorSetOut, "descriptor_set_out", "", "Writes a protobuf FileDescriptorSet containing all the input to FILE")
	flags.StringSliceVar(&op.ProtobufPlugins, "pbplugin", []string{}, "Specifies a protobuf plugin executable to use.")
	flags.StringSliceVar(&op.Plugins, "plugin", []string{}, "Specifies a plugin executable to use.")
	_ = flags.Parse(os.Args[1:])
	targets := flags.Args()
	for x, t := range targets {
		targets[x] = target.Normalize(t)
	}

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
		Files:      targets,
		DumpTokens: op.DumpTokens,
		DumpTree:   op.DumpTree,
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

	if op.DescriptorSetOut != "" {
		fds, err := out.Image.ToFileDescriptorSet()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		bytes, err := proto.Marshal(fds)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		if err = os.WriteFile(op.DescriptorSetOut, bytes, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	for _, plugin := range op.ProtobufPlugins {
		binary, parameters, _ := strings.Cut(plugin, ":")

		fds, err := out.Image.ToFileDescriptorSet()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		request := pluginpb.CodeGeneratorRequest{
			ProtoFile:       fds.File,
			FileToGenerate:  targets,
			CompilerVersion: &pluginpb.Version{},
			Parameter:       &parameters,
		}
		requestBytes, err := proto.Marshal(&request)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		var pluginOut bytes.Buffer
		var pluginErr bytes.Buffer

		binary, err = exec.LookPath(binary)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		cmd := exec.Command(binary)
		cmd.Stdin = bytes.NewReader(requestBytes)
		cmd.Stdout = &pluginOut
		cmd.Stderr = &pluginErr

		err = cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", pluginErr.String())
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		response := pluginpb.CodeGeneratorResponse{}
		err = proto.Unmarshal(pluginOut.Bytes(), &response)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		if response.Error != nil {
			fmt.Fprintln(os.Stderr, response.Error)
			os.Exit(1)
		}

		for _, responseFile := range response.File {
			filename := path.Join(output, *responseFile.Name)
			if err = os.MkdirAll(filepath.Dir(filename), 0770); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			if err = os.WriteFile(path.Join(output, *responseFile.Name), []byte(*responseFile.Content), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		}
	}

	for _, plugin := range op.Plugins {
		name, parameters, _ := strings.Cut(plugin, ":")

		if name != "mgdl-gen-go" {
			fmt.Fprintln(os.Stderr, "Only the mgdl-gen-go plugin is supported, for now")
			os.Exit(1)
		}

		// TODO 2023.12.30: an executable interface like the protobuf one, but passing an idl.Image
		files, err := mgdl_gen_go.Embed(parameters, out.Image, targets)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		for _, responseFile := range files {
			filename := path.Join(output, *responseFile.Name)
			if err = os.MkdirAll(filepath.Dir(filename), 0770); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			if err = os.WriteFile(path.Join(output, *responseFile.Name), []byte(*responseFile.Content), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
		}
	}

	fmt.Println(out.Image)
	fmt.Println(output)
}
