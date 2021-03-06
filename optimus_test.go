// +build !appengine

package optimus

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/pjebs/jsonerror"
)

func GenerateSeed() (*Optimus, uint8, error) {
	baseURL := "http://primes.utm.edu/lists/small/millions/primes%d.zip"

	//Generate Random number between 1-50
	b_49 := *big.NewInt(49)
	n, _ := rand.Int(rand.Reader, &b_49)
	i_n := n.Uint64() + 1

	//Download zip file
	finalUrl := fmt.Sprintf(baseURL, i_n)
	log.Printf("Using file: %s", finalUrl)

	client := &http.Client{}
	resp, err := client.Get(finalUrl)
	if err != nil {
		return nil, uint8(i_n), jsonerror.New(1, "Could not generate seed", err.Error())
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, uint8(i_n), jsonerror.New(1, "Could not generate seed", err.Error())
	}

	r, err := zip.NewReader(bytes.NewReader(body), resp.ContentLength)
	if err != nil {
		return nil, uint8(i_n), jsonerror.New(1, "Could not generate seed", err.Error())
	}

	zippedFile := r.File[0]

	src, err := zippedFile.Open() //src contains ReaderCloser
	if err != nil {
		return nil, uint8(i_n), jsonerror.New(1, "Could not generate seed", err.Error())
	}
	defer src.Close()

	//Create a Byte Slice
	buf := new(bytes.Buffer)
	noOfBytes, _ := buf.ReadFrom(src)
	b := buf.Bytes() //Byte Slice

	//Randomly pick a character position
	start := 67 // Each zip file has an introductory header which is not relevant until the 67th character
	end := noOfBytes

	b_end := *big.NewInt(int64(end) - int64(start))
	n, _ = rand.Int(rand.Reader, &b_end)
	randomPosition := n.Uint64() + uint64(start)

	min := randomPosition - 9
	max := randomPosition + 9

	if min < uint64(start) {
		min = uint64(start)
	}

	if max > uint64(end) {
		max = uint64(end)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(b[min:max]))) //Input
	scanner.Split(bufio.ScanWords)

	var selectedNumbers []uint64
	for scanner.Scan() {
		p, _ := strconv.ParseUint(scanner.Text(), 10, 64)
		selectedNumbers = append(selectedNumbers, p)
	}

	//Not perfect but good enough

	var selectedPrime uint64
	length := len(selectedNumbers)
	if length > 2 {
		//Pick middle number

		//Check if length is even number
		//Check if round is odd or even
		var odd bool
		if length&1 != 0 {
			odd = true //odd
		} else {
			odd = false //even
		}

		if odd {
			selectedPrime = selectedNumbers[length/2]
		} else {

			r := *big.NewInt(1)
			rn, _ := rand.Int(rand.Reader, &r)
			if rn.Uint64() == 0 {
				selectedPrime = selectedNumbers[length/2]
			} else {
				selectedPrime = selectedNumbers[length/2-1]
			}
		}
	} else {
		//Pick largest number
		largest := selectedNumbers[0]

		for _, value := range selectedNumbers {
			if value > largest {
				largest = value
			}
		}

		selectedPrime = largest
	}

	//Calculate Mod Inverse for selectedPrime
	selectedPrime64 := int64(selectedPrime)
	if selectedPrime != uint64(selectedPrime64) {
		return nil, uint8(i_n), jsonerror.New(1, "Could not generate seed", "Prime number found by generator is too large to calculate the ModInverse. This is a limitation in math/big package. Try the generator again.")
	}
	modInverse := ModInverse(selectedPrime64)

	//Generate Random Integer less than MAX_INT
	upper := *big.NewInt(int64(MAX_INT - 2))
	rand, _ := rand.Int(rand.Reader, &upper)
	randomNumber := rand.Uint64() + 1

	o := New(selectedPrime, modInverse, randomNumber)
	return &o, uint8(i_n), nil
}

// Tests if the encoding process correctly decodes the id back to the original.
func TestEncoding(t *testing.T) {
	for i := 0; i < 5; i++ { //How many times we want to run GenerateSeed()
		o, _, _ := GenerateSeed()

		c := 10
		h := 100 //How many random numbers to select in between 0-c and (MAX_INT-c) - MAX-INT

		var y []uint64 //Stores all the values we want to run encoding tests on

		for t := 0; t < c; t++ {
			y = append(y, uint64(t))
		}

		//Generate Random numbers
		for t := 0; t < h; t++ {
			upper := *big.NewInt(int64(MAX_INT - 2*uint64(c)))
			rand, _ := rand.Int(rand.Reader, &upper)
			randomNumber := rand.Uint64() + uint64(c)

			y = append(y, randomNumber)
		}

		for t := MAX_INT; t >= MAX_INT-uint64(c); t-- {
			y = append(y, t)
		}

		t.Logf("Prime: %d ModInverse: %d Random: %d", o.Prime(), o.ModInverse(), o.Random())
		for _, value := range y {
			orig := value
			hashed := o.Encode(value)
			unhashed := o.Decode(hashed)

			if orig != unhashed {
				t.Errorf("%d: %d -> %d - FAILED", orig, hashed, unhashed)
			} else {
				t.Logf("%d: %d -> %d - PASSED", orig, hashed, unhashed)
				// log.Printf("%d: %d -> %d - PASSED", orig, hashed, unhashed)
			}
		}

	}
}
