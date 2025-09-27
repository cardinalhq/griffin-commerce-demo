package main

import "github.com/cardinalhq/griffin-commerce-demo/common"

// Export internal functions for testing
func InitProducts() {
	productDB = common.NewMockDB()
}

func GetProductDB() *common.MockDB {
	return productDB
}
