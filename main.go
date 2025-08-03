package main

import (
	"context"
	"fmt"
	"os"

	"bloggerout/internal/convert"

	"github.com/spf13/cobra"
)

func main() {
	// listCmd := &cobra.Command{
	// 	Use:   "list",
	// 	Short: "List the contents of the Blogger Takeout file",
	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		if len(args) == 0 {
	// 			fmt.Println("Usage: bloggerout list <path-to-blogger-takeout-file>")
	// 			os.Exit(1)
	// 		}

	// 		ctx := context.Background()
	// 		takeoutData, err := takeout.ReadTakeout(ctx, args)
	// 		if err != nil {
	// 			fmt.Printf("Error reading takeout: %v\n", err)
	// 			os.Exit(1)
	// 		}

	// 		takeoutData.ListBlogs()
	// 	},
	// }

	rootCmd := &cobra.Command{
		Use:   "bloggerout",
		Short: "Bloggerout converts Blogger Takeout files to Hugo-compatible files",
	}

	// rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(convert.ConverCommand())

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
