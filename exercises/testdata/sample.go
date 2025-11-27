package testdata

import (
	"fmt"
	"time"
)

// Sample functions for testing the search MCP server

func HelloWorld() {
	fmt.Println("Hello, World!")
}

func Add(a, b int) int {
	return a + b
}

func Multiply(x, y int) int {
	return x * y
}

func ProcessData(data []string) {
	for _, item := range data {
		fmt.Println(item)
	}
}

func Calculate(n int) int {
	result := 0
	for i := 0; i < n; i++ {
		result += i
	}
	return result
}

func WaitAndPrint(duration time.Duration, message string) {
	time.Sleep(duration)
	fmt.Println(message)
}

type User struct {
	Name  string
	Email string
	Age   int
}

func NewUser(name, email string, age int) *User {
	return &User{
		Name:  name,
		Email: email,
		Age:   age,
	}
}

func (u *User) Greet() string {
	return fmt.Sprintf("Hello, I'm %s", u.Name)
}

func (u *User) IsAdult() bool {
	return u.Age >= 18
}

func Sum(numbers ...int) int {
	total := 0
	for _, num := range numbers {
		total += num
	}
	return total
}

func Filter(items []int, predicate func(int) bool) []int {
	result := []int{}
	for _, item := range items {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

func Map(items []int, transform func(int) int) []int {
	result := make([]int, len(items))
	for i, item := range items {
		result[i] = transform(item)
	}
	return result
}
