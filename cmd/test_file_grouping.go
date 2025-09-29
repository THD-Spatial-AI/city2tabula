package main

// import (
// 	"fmt"
// 	"log"
// 	"City2TABULA/internal/config"
// )

// func main() {
// 	// Get all sql files in the sql directories
// 	// and print them in order
// 	// to verify the grouping logic

// 	scripts, err := config.LoadSQLScripts()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	fmt.Println("=== Main Scripts ===")
// 	for _, script := range scripts.MainScripts {
// 		fmt.Println(script)
// 	}

// 	fmt.Println("\n=== Supplementary Scripts ===")
// 	for _, script := range scripts.SupplementaryScripts {
// 		fmt.Println(script)
// 	}

// 	fmt.Println("\n=== Schema Scripts ===")
// 	for _, script := range scripts.TableScripts {
// 		fmt.Println(script)
// 	}

// 	fmt.Println("\n=== Function Scripts ===")
// 	for _, script := range scripts.FunctionScripts {
// 		fmt.Println(script)
// 	}
// }
