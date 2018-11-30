package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim/internal/uvm"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	kernelArgsArgName           = "kernel-args"
	rootFSTypeArgName           = "root-fs-type"
	vpMemMaxCountArgName        = "vpmem-max-count"
	vpMemMaxSizeArgName         = "vpmem-max-size"
	cpusArgName                 = "cpus"
	memoryArgName               = "memory"
	disallowOvercommitArgName   = "disallow-overcommit"
	enableDeferredCommitArgName = "enable-deferred-commit"
	measureArgName              = "measure"
	parallelArgName             = "parallel"
	countArgName                = "count"
)

func main() {
	app := cli.NewApp()
	app.Name = "uvmboot"
	app.Usage = "Boot a utility VM and collect dmesg output"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  kernelArgsArgName,
			Value: "",
			Usage: "Additional arguments to pass to the kernel",
		},
		cli.UintFlag{
			Name:  rootFSTypeArgName,
			Value: 0,
			Usage: "0 to boot from initrd, 1 to boot from VHD",
		},
		cli.UintFlag{
			Name:  vpMemMaxCountArgName,
			Value: 64,
			Usage: "Number of VPMem devices on the UVM",
		},
		cli.Uint64Flag{
			Name:  vpMemMaxSizeArgName,
			Value: 4 * 1024,
			Usage: "Size of each VPMem device, in MB",
		},
		cli.Uint64Flag{
			Name:  cpusArgName,
			Value: 2,
			Usage: "Number of CPUs on the UVM",
		},
		cli.UintFlag{
			Name:  memoryArgName,
			Value: 1024,
			Usage: "Amount of memory on the UVM, in MB",
		},
		cli.BoolFlag{
			Name:  disallowOvercommitArgName,
			Usage: "Disable memory overcommit on the UVM",
		},
		cli.BoolFlag{
			Name:  enableDeferredCommitArgName,
			Usage: "Enable deferred commit on the UVM",
		},
		cli.BoolFlag{
			Name:  measureArgName,
			Usage: "Measure wall clock time of the UVM run",
		},
		cli.BoolFlag{
			Name:  parallelArgName,
			Usage: "Run the UVMs in parallel instead of sequentially",
		},
		cli.IntFlag{
			Name:  countArgName,
			Value: 1,
			Usage: "Number of UVMs to run",
		},
	}

	app.Action = func(c *cli.Context) error {
		rootFSType := uvm.PreferredRootFSType(c.Int(rootFSTypeArgName))
		vpMemMaxCount := uint32(c.Uint(vpMemMaxCountArgName))
		vpMemMaxSize := c.Uint64(vpMemMaxSizeArgName) * 1024 * 1024 // convert from MB to bytes
		cpus := c.Uint64(cpusArgName)
		memory := c.Uint64(memoryArgName) * 1024 * 1024 // convert from MB to bytes
		allowOvercommit := !c.Bool(disallowOvercommitArgName)
		enableDeferredCommit := c.Bool(enableDeferredCommitArgName)

		runCount := c.Int(countArgName)

		var wg sync.WaitGroup
		wg.Add(runCount)

		runFunc := func(i int) {
			options := uvm.UVMOptions{
				OperatingSystem:     "linux",
				KernelBootOptions:   c.String(kernelArgsArgName),
				PreferredRootFSType: &rootFSType,
				VPMemDeviceCount:    &vpMemMaxCount,
				VPMemSizeBytes:      &vpMemMaxSize,
				Resources: &specs.WindowsResources{
					CPU: &specs.WindowsCPUResources{
						Count: &cpus,
					},
					Memory: &specs.WindowsMemoryResources{
						Limit: &memory,
					},
				},
				AllowOvercommit:      &allowOvercommit,
				EnableDeferredCommit: &enableDeferredCommit,
			}

			// log.Infof("[%d] Starting", i)

			if err := run(&options); err != nil {
				// log.Errorf("[%d] %s", i, err)
			}

			// log.Infof("[%d] Finished", i)

			wg.Done()
		}

		start := time.Now()

		for i := 0; i < runCount; i++ {
			if c.Bool(parallelArgName) {
				go runFunc(i)
			} else {
				runFunc(i)
			}
		}

		wg.Wait()

		if c.Bool(measureArgName) {
			fmt.Println("Elapsed time:", time.Since(start))
		}

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(options *uvm.UVMOptions) error {
	uvm, err := uvm.Create(options)
	if err != nil {
		return err
	}
	defer uvm.Close()

	if err := uvm.Start(); err != nil {
		return err
	}

	if err := uvm.Wait(); err != nil {
		return err
	}

	return nil
}
