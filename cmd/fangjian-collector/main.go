package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appcommunitymarket "github.com/sine-io/propulse/internal/application/communitymarket"
	infrastructurefangjian "github.com/sine-io/propulse/internal/infrastructure/fangjian"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("fangjian-collector", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("output", "data/fangjian", "archive root")
	community := flags.String("community", "all", "all, mingquan, or qinhe")
	baseURL := flags.String("api-base", envOr("FANGJIAN_API_BASE", "https://dzfj.elmleaf.com.cn/api"), "Fangjian API base URL")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	authorization := strings.TrimSpace(os.Getenv("FANGJIAN_AUTHORIZATION"))
	ak := strings.TrimSpace(os.Getenv("FANGJIAN_AK"))
	version := strings.TrimSpace(os.Getenv("FANGJIAN_VERSION"))
	missing := make([]string, 0, 3)
	credentials := []struct{ name, value string }{
		{"FANGJIAN_AUTHORIZATION", authorization}, {"FANGJIAN_AK", ak}, {"FANGJIAN_VERSION", version},
	}
	for _, credential := range credentials {
		if credential.value == "" {
			missing = append(missing, credential.name)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "missing required environment variables: %s\n", strings.Join(missing, ", "))
		return 1
	}
	client, err := infrastructurefangjian.NewClient(infrastructurefangjian.ClientConfig{
		BaseURL: *baseURL, Authorization: authorization, AK: ak, Version: version,
		MinInterval: 200 * time.Millisecond, MaxAttempts: 3,
	}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	archive := infrastructurefangjian.NewFileArchive(*output)
	runStartedAt := time.Now().UTC()
	collector := appcommunitymarket.NewCollector(client, archive, func() time.Time { return runStartedAt })
	selected := make([]appcommunitymarket.FangjianCommunityConfig, 0, 2)
	for _, item := range appcommunitymarket.DefaultFangjianCommunities {
		if *community == "all" || *community == item.Slug {
			selected = append(selected, item)
		}
	}
	if len(selected) == 0 {
		fmt.Fprintln(os.Stderr, "community must be all, mingquan, or qinhe")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for _, item := range selected {
		path, bundle, err := collector.Collect(ctx, item)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s collection failed: %v\n", item.Slug, err)
			return 1
		}
		fmt.Printf("community=%s archive=%s listings=%d transactions=%d adjustments=%d\n", item.Slug, path, len(bundle.Listings), len(bundle.Transactions), len(bundle.Adjustments))
	}
	return 0
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
