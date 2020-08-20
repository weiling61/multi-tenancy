/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package kubectl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/multi-tenancy/benchmarks/kubectl-mtb/internal/reporter"
	"sigs.k8s.io/multi-tenancy/benchmarks/kubectl-mtb/pkg/benchmark"
	"sigs.k8s.io/multi-tenancy/benchmarks/kubectl-mtb/test"
	"sigs.k8s.io/multi-tenancy/benchmarks/kubectl-mtb/types"
)

var benchmarkRunOptions = types.RunOptions{}

var runCmd = &cobra.Command{
	Use:   "run <resource>",
	Short: "Run the Multi-Tenancy Benchmarks",

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("Please specify a resource")
		}
		if !supportedResourceNames.Has(args[0]) {
			return fmt.Errorf("Please specify a valid resource")
		}
		err := validateFlags(cmd)
		if err != nil {
			return err
		}

		filterBenchmarks(cmd)

		return nil
	},

	Run: func(cmd *cobra.Command, args []string) {
		cmdutil.CheckErr(runTests(cmd, args))
	},
}

func initConfig() error {
	kubecfgFlags := genericclioptions.NewConfigFlags(false)
	config, err := kubecfgFlags.ToRESTConfig()
	if err != nil {
		return err
	}

	// create the K8s clientset
	benchmarkRunOptions.KClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	tenantConfig := config
	tenantConfig.Impersonate.UserName = benchmarkRunOptions.Tenant

	// create the tenant clientset
	benchmarkRunOptions.TClient, err = kubernetes.NewForConfig(tenantConfig)
	if err != nil {
		return err
	}

	return err
}

func reportSuiteWillBegin(suiteSummary *reporter.SuiteSummary, reportersArray []reporter.Reporter) {
	for _, reporter := range reportersArray {
		reporter.SuiteWillBegin(suiteSummary)
	}
}

func reportTestWillRun(testSummary *reporter.TestSummary, reportersArray []reporter.Reporter) {
	for _, reporter := range reportersArray {
		reporter.TestWillRun(testSummary)
	}
}

func reportSuiteDidEnd(suiteSummary *reporter.SuiteSummary, reportersArray []reporter.Reporter) {
	for _, reporter := range reportersArray {
		reporter.SuiteDidEnd(suiteSummary)
	}
}

func removeBenchmarksWithIDs(ids []string) {
	temp := []*benchmark.Benchmark{}
	for _, benchmark := range benchmarks {
		found := false
		for _, id := range ids {
			if benchmark.ID == id {
				found = true
			}
		}

		if !found {
			temp = append(temp, benchmark)
		}
	}
	benchmarks = temp
}

// Validation of the flag inputs
func validateFlags(cmd *cobra.Command) error {
	benchmarkRunOptions.Tenant, _ = cmd.Flags().GetString("as")
	if benchmarkRunOptions.Tenant == "" {
		return fmt.Errorf("username must be set via --as")
	}

	benchmarkRunOptions.TenantNamespace, _ = cmd.Flags().GetString("namespace")
	if benchmarkRunOptions.TenantNamespace == "" {
		return fmt.Errorf("tenant namespace must be set via --namespace or -n")
	}

	err := initConfig()
	if err != nil {
		return err
	}

	_, err = benchmarkRunOptions.KClient.CoreV1().Namespaces().Get(context.TODO(), benchmarkRunOptions.TenantNamespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("tenantnamespace is not a valid namespace")
	}

	return nil
}

func runTests(cmd *cobra.Command, args []string) error {

	benchmarkRunOptions.Label, _ = cmd.Flags().GetString("labels")

	// Get reporters from the user
	reporterFlag, _ := cmd.Flags().GetString("out")
	reporters := strings.Split(reporterFlag, ",")
	reportersArray, err := reporter.GetReporters(reporters)
	if err != nil {
		return err
	}

	// Get benchmark ids from the user to skip them
	skipFlag, _ := cmd.Flags().GetString("skip")
	skipIDs := strings.Split(skipFlag, ",")
	removeBenchmarksWithIDs(skipIDs)

	suiteSummary := &reporter.SuiteSummary{
		Suite:                test.BenchmarkSuite,
		NumberOfTotalTests:   len(benchmarks),
		TenantAdminNamespace: benchmarkRunOptions.TenantNamespace,
	}

	suiteStartTime := time.Now()
	reportSuiteWillBegin(suiteSummary, reportersArray)

	for _, b := range benchmarks {

		ts := &reporter.TestSummary{
			Benchmark: b,
		}

		err := ts.SetDefaults()
		if err != nil {
			return err
		}

		startTest := time.Now()

		//Run Prerun
		err = b.PreRun(benchmarkRunOptions)
		if err != nil {
			suiteSummary.NumberOfFailedValidations++
			ts.Validation = false
			ts.ValidationError = err
			b.Status = "Error"
		}

		// Check PreRun status
		if ts.Validation {
			err = b.Run(benchmarkRunOptions)
			if err != nil {
				suiteSummary.NumberOfFailedTests++
				ts.Test = false
				ts.TestError = err
				b.Status = "Fail"
			} else {
				suiteSummary.NumberOfPassedTests++
				b.Status = "Pass"
			}
		}

		// Check Run status
		if ts.Test {
			if b.PostRun != nil {
				err = b.PostRun(benchmarkRunOptions)
				if err != nil {
					fmt.Print(err.Error())
				}
			}
		}
		elapsed := time.Since(startTest)
		ts.RunTime = elapsed
		reportTestWillRun(ts, reportersArray)
	}

	suiteElapsedTime := time.Since(suiteStartTime)
	suiteSummary.RunTime = suiteElapsedTime
	suiteSummary.NumberOfSkippedTests = test.BenchmarkSuite.Totals() - len(benchmarks)
	reportSuiteDidEnd(suiteSummary, reportersArray)

	return nil
}

func newRunCmd() *cobra.Command {
	runCmd.Flags().StringP("namespace", "n", "", "(required) tenant namespace")
	runCmd.Flags().String("as", "", "(required) user name to impersonate")
	runCmd.Flags().StringP("out", "o", "default", "(optional) output reporters (default, policyreport)")
	runCmd.Flags().StringP("skip", "s", "", "(optional) benchmark IDs to skip")
	runCmd.Flags().StringP("labels", "l", "", "(optional) labels")

	return runCmd
}
