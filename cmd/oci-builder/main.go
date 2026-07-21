// oci-builder creates a secure OCI Image Layout from a compiled Go binary.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/CYPT71/secure-oci-base/internal/oci"
)

func Run(args []string) int { return run(args, os.Stdout, os.Stderr) }
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("oci-builder", flag.ContinueOnError)
	fs.SetOutput(stderr)
	binary := fs.String("binary", "", "path to statically compiled executable")
	output := fs.String("output", "", "new OCI layout directory")
	arch := fs.String("arch", "", "OCI architecture (default: host architecture)")
	osName := fs.String("os", "linux", "OCI operating system")
	entrypoint := fs.String("entrypoint", "/app/service", "absolute command path in the image")
	image := fs.String("image", "secure-oci-base", "image name annotation")
	tag := fs.String("tag", "latest", "image tag annotation")
	created := fs.String("created", "1970-01-01T00:00:00Z", "RFC3339 creation time")
	var labels labelFlags
	fs.Var(&labels, "label", "image label (key=value); repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	createdAt, err := time.Parse(time.RFC3339, *created)
	if err != nil {
		fmt.Fprintf(stderr, "invalid -created: %v\n", err)
		return 2
	}
	parsedLabels, err := oci.LabelsFromPairs(labels)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	digest, err := oci.Build(oci.Options{Binary: *binary, Output: *output, Architecture: *arch, OS: *osName, Entrypoint: *entrypoint, ImageName: *image, Tag: *tag, Created: createdAt, Labels: parsedLabels})
	if err != nil {
		fmt.Fprintf(stderr, "oci-builder: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "created OCI layout %s (%s)\n", *output, digest)
	return 0
}

type labelFlags []string

func (l *labelFlags) String() string         { return "" }
func (l *labelFlags) Set(value string) error { *l = append(*l, value); return nil }
func main()                                  { os.Exit(Run(os.Args[1:])) }
