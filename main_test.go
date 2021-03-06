package main

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/google/gopacket/pcapgo"
)

func TestMain(m *testing.M) {
	os.Stderr = os.Stdin // so joincap -v won't pollute the output
	os.Exit(m.Run())
}

const okPcap = "pcap_examples/ok.pcap"

func packetCount(t *testing.T, pcapPath string) uint64 {
	inputFile, err := os.Open(pcapPath)
	if err != nil {
		t.Fatal(err)
	}
	defer inputFile.Close()

	reader, err := pcapgo.NewReader(inputFile)
	if err != nil {
		t.Fatal(err)
	}

	var packetCount uint64
	for {
		_, _, err = reader.ReadPacketData()
		if err == io.EOF {
			return packetCount
		} else if err != nil {
			t.Fatal(err)
		}
		packetCount++
	}
}

// TestHelperPacketCount test the helper function packetCount
func TestHelperPacketCount(t *testing.T) {
	// tcpdump -r pcap_examples/ok.pcap -qn | wc -l
	if packetCount(t, okPcap) != 851 {
		t.FailNow()
	}
}

func isTimeOrdered(pcapPath string) (bool, error) {
	inputFile, err := os.Open(pcapPath)
	if err != nil {
		return false, err
	}
	defer inputFile.Close()

	reader, err := pcapgo.NewReader(inputFile)
	if err != nil {
		return false, err
	}

	var previousTime int64
	for {
		_, capInfo, err := reader.ReadPacketData()
		if err == io.EOF {
			return true, nil
		} else if err != nil {
			return false, err
		}

		currentTime := capInfo.Timestamp.UnixNano()

		if currentTime < previousTime {
			return false, nil
		}

		previousTime = currentTime
	}
}

// TestHelperIsTimeOrderedTrue test the helper function isTimeOrdered for positive value
func TestHelperIsTimeOrderedTrue(t *testing.T) {
	isOutputOrdered, err := isTimeOrdered(okPcap)
	if err != nil {
		t.Fatal(err)
	}
	if !isOutputOrdered {
		t.FailNow()
	}
}

// TestHelperIsTimeOrderedTrue test the helper function isTimeOrdered for negative value
func TestHelperIsTimeOrderedFalse(t *testing.T) {
	isOutputOrdered, err := isTimeOrdered("pcap_examples/out_of_order.pcap")
	if err != nil {
		t.Fatal(err)
	}
	if isOutputOrdered {
		t.FailNow()
	}
}

func testIsOrdered(t *testing.T, pcapPath string) {
	isOutputOrdered, err := isTimeOrdered(pcapPath)
	if err != nil {
		t.Fatal(err)
	}
	if !isOutputOrdered {
		t.Fatal("out of order")
	}
}

// TestCount packet count of merged pcap
// should be the sum of the packet counts of the
// input pcaps
func TestCount(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		okPcap, okPcap})

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap)*2 {
		t.FailNow()
	}
}

// TestOrder all packets in merged pacap should
// be ordered by time
func TestOrder(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		okPcap, okPcap})

	testIsOrdered(t, okPcap)
	testIsOrdered(t, outputFile.Name())
}

// TestIgnoreInputFileCorruptGlobalHeader merging pcap with
// a corrupt global header should be ignored
func TestIgnoreInputFileCorruptGlobalHeader(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/bad_global.pcap"})

	if packetCount(t, outputFile.Name()) != 0 {
		t.FailNow()
	}
}

// TestIgnorePacketWithCorruptHeader packet with corrupt header should be ignored
func TestIgnorePacketWithCorruptHeader(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		okPcap, "pcap_examples/bad_first_header.pcap"})

	testIsOrdered(t, outputFile.Name())

	// bad_first_header.pcap is ok.pcap with its first packet header ruined
	if (packetCount(t, okPcap)*2)-1 != packetCount(t, outputFile.Name()) {
		t.FailNow()
	}
}

// TestIgnoreTruncatedPacket truncated packet (EOF) should be ignored
func TestIgnoreTruncatedPacketEOF(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/unexpected_eof_on_second_packet.pcap"})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != 1 {
		t.FailNow()
	}
}

// TestIgnoreEmptyPcap pcap without packets should be ignored
func TestIgnoreEmptyPcap(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		okPcap, "pcap_examples/no_packets.pcap"})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap) {
		t.FailNow()
	}
}

// TestIgnoreInputFileTruncatedGlobalHeader pcap without full global header (< 24 bytes) should be ignored
func TestIgnoreInputFileTruncatedGlobalHeader(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		okPcap, "pcap_examples/partial_global_header.pcap"})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap) {
		t.FailNow()
	}
}

// TestIgnoreInputFileTruncatedFirstPacketHeader pcap without full first packet header (24 < size < 40 bytes) should be ignored
func TestIgnoreInputFileTruncatedFirstPacketHeader(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/partial_first_header.pcap", okPcap})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap) {
		t.FailNow()
	}
}

// TestIgnoreInputFileDoesntExists non existing input files should be ignored
func TestIgnoreInputFileDoesNotExists(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"/nothing/here", okPcap, "or_here"})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap) {
		t.FailNow()
	}
}

// TestIgnoreInputFileIsDirectory directory as input file should be ignored
func TestIgnoreInputFileIsDirectory(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples", okPcap})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap) {
		t.FailNow()
	}
}

// TestIgnoreGarbageEndingOfPcap garbage at end of pcap should be ignored (this kills tcpslice)
func TestIgnoreGarbageEndingOfPcap(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/bad_end.pcap", okPcap})

	testIsOrdered(t, outputFile.Name())

	// bad_end.pcap is ok.pcap with the last packet header ruined and garbage appended to it
	if packetCount(t, outputFile.Name()) != (packetCount(t, okPcap)*2)-1 {
		t.FailNow()
	}
}

// TestGzippedPcap gzipped pcap should merge just fine (this kills tcpslice)
func TestGzippedPcap(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/ok.pcap.gz", okPcap})

	testIsOrdered(t, outputFile.Name())

	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap)*2 {
		t.FailNow()
	}
}

// TestIgnoreToSmallSnaplen snaplen should be ignored and we use our own snaplen
func TestIgnoreTooSmallSnaplen(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/very_small_snaplen.pcap"})

	// snaplen is edited to be way to small
	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap) {
		t.FailNow()
	}
}

// TestIgnorePacketsWithTimeEarlierThanFirst packets with timestamp smaller than the
// first packet should be ignored
func TestIgnorePacketsWithTimeEarlierThanFirst(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/second_packet_time_is_too_small.pcap"})

	// the second packet is edited to have 1970 date...
	if packetCount(t, outputFile.Name()) != packetCount(t, okPcap)-1 {
		t.FailNow()
	}
}

// TestPrintVersion tests that the version is printed okay
func TestPrintVersion(t *testing.T) {
	savedStdout := os.Stdout
	defer func() { os.Stdout = savedStdout }()

	stdoutTmpFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	filename := stdoutTmpFile.Name()
	defer os.Remove(filename)

	os.Stdout = stdoutTmpFile
	joincap([]string{"joincap", "-V"})
	stdoutTmpFile.Close()

	stdoutBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(stdoutBytes)) != "joincap v"+version {
		t.Fatal(strings.TrimSpace(string(stdoutBytes)), "joincap v"+version)
	}
}

// TestPrintHelp tests that the help is printed okay
func TestPrintHelp(t *testing.T) {
	savedStdout := os.Stdout
	defer func() { os.Stdout = savedStdout }()

	stdoutTmpFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	filename := stdoutTmpFile.Name()
	defer os.Remove(filename)

	os.Stdout = stdoutTmpFile
	joincap([]string{"joincap", "-h"})
	stdoutTmpFile.Close()

	stdoutBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	help := strings.TrimSpace(string(stdoutBytes))

	if !strings.HasPrefix(help, "Usage:") {
		t.FailNow()
	}
}

// TestExitOnUnknownFlag tests exit on unknown cli flag
func TestExitOnUnknownFlag(t *testing.T) {
	err := joincap([]string{"joincap", "--banana"})
	if err == nil {
		t.Fatal("Shouldn't exited without an error")
	}
	if !strings.Contains(err.Error(), "unknown flag") ||
		!strings.Contains(err.Error(), "banana") {
		t.FailNow()
	}
}

// TestWriteToNonExistingDirectory test writing to file in non existing directory
func TestWriteToNonExistingDirectory(t *testing.T) {
	err := joincap([]string{"joincap", "-v", "-w", "/banana/papaya.pcap"})
	if err == nil {
		t.Fatal("Shouldn't exited without an error")
	}
	if !strings.HasPrefix(err.Error(), "cannot open") {
		t.FailNow()
	}
}

// TestExitOnDifferentLinkTypes test cannot merge different linktypes
func TestExitOnDifferentLinkTypes(t *testing.T) {
	outputFile, err := ioutil.TempFile("", "joincap_output_")
	if err != nil {
		t.Fatal(err)
	}
	outputFile.Close()
	defer os.Remove(outputFile.Name())

	err = joincap([]string{"joincap",
		"-v", "-w", outputFile.Name(),
		"pcap_examples/ok.pcap", "pcap_examples/linktype_unknown.pcap"})

	if err == nil {
		t.Fatal("Shouldn't exited without an error")
	}
	if !strings.Contains(err.Error(), "different linktypes") {
		t.FailNow()
	}
}

func Benchmark(b *testing.B) {
	for n := 0; n < b.N; n++ {
		joincap([]string{"joincap",
			"-w", "/dev/null",
			"pcap_examples/ok.pcap", "pcap_examples/ok.pcap"})
	}
}
