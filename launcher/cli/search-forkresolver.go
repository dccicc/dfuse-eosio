package cli

import (
	"fmt"

	eosSearch "github.com/dfuse-io/dfuse-eosio/search"
	"github.com/dfuse-io/dlauncher/launcher"
	forkresolverApp "github.com/dfuse-io/search/app/forkresolver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	// Search Fork Resolver
	launcher.RegisterApp(&launcher.AppDef{
		ID:          "search-forkresolver",
		Title:       "Search fork resolver",
		Description: "Search forks",
		MetricsID:   "forkresolver",
		Logger:      launcher.NewLoggingDef("github.com/dfuse-io/search/(forkresolver|app/forkresolver).*", nil),
		RegisterFlags: func(cmd *cobra.Command) error {
			cmd.Flags().String("search-forkresolver-grpc-listen-addr", ForkresolverServingAddr, "Address to listen for incoming gRPC requests")
			cmd.Flags().String("search-forkresolver-http-listen-addr", ForkresolverHTTPServingAddr, "Address to listen for incoming HTTP requests")
			cmd.Flags().String("search-forkresolver-indices-path", "{dfuse-data-dir}/search/forkresolver", "Location for inflight indices")
			return nil
		},
		FactoryFunc: func(modules *launcher.RuntimeModules) (launcher.App, error) {
			mapper, err := eosSearch.NewEOSBlockMapper(
				viper.GetString("search-common-dfuse-events-action-name"),
				viper.GetBool("search-common-dfuse-events-unrestricted"),
				viper.GetString("search-common-action-filter-on-expr"),
				viper.GetString("search-common-action-filter-out-expr"),
			)
			if err != nil {
				return nil, fmt.Errorf("unable to create EOS block mapper: %w", err)
			}

			return forkresolverApp.New(&forkresolverApp.Config{
				ServiceVersion:  viper.GetString("search-common-mesh-service-version"),
				GRPCListenAddr:  viper.GetString("search-forkresolver-grpc-listen-addr"),
				HttpListenAddr:  viper.GetString("search-forkresolver-http-listen-addr"),
				PublishInterval: viper.GetDuration("search-common-mesh-publish-interval"),
				IndicesPath:     viper.GetString("search-forkresolver-indices-path"),
				BlocksStoreURL:  viper.GetString("common-blocks-store-url"),
			}, &forkresolverApp.Modules{
				Dmesh:       modules.SearchDmeshClient,
				BlockMapper: mapper,
			}), nil
		},
	})
}
