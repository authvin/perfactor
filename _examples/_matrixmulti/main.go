package main

import "math/rand"

func main() {
	seed := 42
	size := 1000
	matrix1 := initMatrix(seed, size)
	matrix2 := initMatrix(seed+1, size)

	matrix3 := matrixMulti(matrix1, matrix2)
	println(len(matrix3))
}

func matrixMulti(a, b [][]float32) [][]float32 {
	size := len(a)
	if size == 0 {
		return nil
	}
	if len(a[0]) != size {
		return nil
	}

	c := make([][]float32, len(a))
	for i := range c {
		c[i] = make([]float32, len(a[0]))
	}
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			for k := 0; k < size; k++ {
				c[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return c
}

func initMatrix(seed, size int) [][]float32 {
	c := make([][]float32, size)
	r := rand.New(rand.NewSource(int64(seed)))
	for i := 0; i < size; i++ {
		arr := make([]float32, size)
		for j := 0; j < size; j++ {
			arr[j] = r.Float32()
		}
		c[i] = arr
	}
	return c
}
