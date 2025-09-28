package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "griffin",
	Short: "Griffin Commerce Demo - Microservices E-commerce Platform",
	Long:  `Griffin Commerce Demo is a microservices-based e-commerce platform that demonstrates modern cloud-native architecture patterns.`,
}

func Execute() error {
	return rootCmd.Execute()
}
