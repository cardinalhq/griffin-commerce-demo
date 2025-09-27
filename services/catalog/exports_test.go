package main

import "github.com/griffincommerce/demo/common"

// Export internal functions for testing
func InitProducts() {
	productDB = common.NewMockDB()
}

func GetProductDB() *common.MockDB {
	return productDB
}
