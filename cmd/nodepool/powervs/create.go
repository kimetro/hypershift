package powervs

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/intstr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	hyperv1 "github.com/openshift/hypershift/api/v1alpha1"
	"github.com/openshift/hypershift/cmd/log"
	"github.com/openshift/hypershift/cmd/nodepool/core"
)

type PowerVSPlatformCreateOptions struct {
	SysType    string
	ProcType   string
	Processors string
	Memory     int32
}

func NewCreateCommand(coreOpts *core.CreateNodePoolOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "powervs",
		Short:        "Creates an PowerVS nodepool",
		SilenceUsage: true,
	}
	opts := &PowerVSPlatformCreateOptions{
		SysType:    "s922",
		ProcType:   "shared",
		Processors: "0.5",
		Memory:     32,
	}

	cmd.Flags().StringVar(&opts.SysType, "sys-type", opts.SysType, "System type used to host the instance(e.g: s922, e980, e880). Default is s922")
	cmd.Flags().StringVar(&opts.ProcType, "proc-type", opts.ProcType, "Processor type (dedicated, shared, capped). Default is shared")
	cmd.Flags().StringVar(&opts.Processors, "processors", opts.Processors, "Number of processors allocated. Default is 0.5")
	cmd.Flags().Int32Var(&opts.Memory, "memory", opts.Memory, "Amount of memory allocated (in GB). Default is 32")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT)
		go func() {
			<-sigs
			cancel()
		}()

		if err := coreOpts.CreateNodePool(ctx, opts); err != nil {
			log.Log.Error(err, "Failed to create nodepool")
			os.Exit(1)
		}
	}

	return cmd
}

func (o *PowerVSPlatformCreateOptions) UpdateNodePool(ctx context.Context, nodePool *hyperv1.NodePool, hcluster *hyperv1.HostedCluster, client crclient.Client) error {
	nodePool.Spec.Platform.Type = hyperv1.PowerVSPlatform
	nodePool.Spec.Platform.PowerVS = &hyperv1.PowerVSNodePoolPlatform{
		SystemType:    o.SysType,
		Processors:    intstr.FromString(o.Processors),
		ProcessorType: o.ProcType,
		MemoryGiB:     o.Memory,
	}
	return nil
}

func (o PowerVSPlatformCreateOptions) Type() hyperv1.PlatformType {
	return hyperv1.PowerVSPlatform
}
