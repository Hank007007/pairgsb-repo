// Command dnsscan replays captured upstream responses through the intake
// layer and prints what the intake layer made of each one.
//
// Each file in testdata is decoded with wire.Parse, then screened with
// cache.Screen. A single parser budget is shared by the whole run, the way a
// resolver's per-connection parser would be.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"

	"ptrloop/internal/cache"
	"ptrloop/internal/limits"
	"ptrloop/internal/wire"
)

func main() {
	dir := flag.String("dir", "testdata", "directory holding captured responses")
	flag.Parse()

	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dnsscan: %v\n", err)
		os.Exit(2)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".bin" {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "dnsscan: no .bin files in %s\n", *dir)
		os.Exit(2)
	}

	parser := wire.NewParser()
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(*dir, name))
		if err != nil {
			fmt.Printf("== %s\nerror: %v\n\n", name, err)
			continue
		}
		scan(name, raw, parser)
	}
}

func scan(name string, raw []byte, parser *wire.Parser) {
	fmt.Printf("== %s (%d bytes)\n", name, len(raw))

	msg, err := parser.Parse(raw)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		printBudget(parser.Budget())
		fmt.Println()
		return
	}

	flags := []string{}
	for bit, label := range map[uint16]string{
		wire.FlagQR: "qr", wire.FlagAA: "aa", wire.FlagTC: "tc",
		wire.FlagRD: "rd", wire.FlagRA: "ra",
	} {
		if msg.Header.Flags&bit != 0 {
			flags = append(flags, label)
		}
	}
	sort.Strings(flags)
	fmt.Printf("header: id=%d flags=%v opcode=%d rcode=%d qd=%d an=%d ns=%d ar=%d\n",
		msg.Header.ID, flags, msg.Header.Opcode(), msg.Header.RCODE(),
		msg.Header.QDCount, msg.Header.ANCount, msg.Header.NSCount, msg.Header.ARCount)

	for _, q := range msg.Questions {
		fmt.Printf("question: %s %s class=%d\n", q.Name, typeName(q.Type), q.Class)
	}
	printSection("answer", msg.Answers)
	printSection("authority", msg.Authorities)
	printSection("additional", msg.Additionals)

	if len(msg.Questions) > 0 {
		d := cache.Screen(msg, msg.Questions[0].Name)
		fmt.Printf("cache: owner=%s accepted=%d rejected=%d\n", d.Owner, len(d.Cacheable), len(d.Rejected))
		for _, r := range d.Rejected {
			fmt.Printf("  reject %s\n", r)
		}
		for _, e := range d.Cacheable {
			mark := ""
			if e.TTL.Clamped {
				mark = " clamped"
			}
			fmt.Printf("  accept %s %s ttl=%d%s\n", e.Owner, typeName(e.Type), e.TTL.Seconds, mark)
		}
	}

	printBudget(parser.Budget())
	fmt.Println()
}

func printSection(label string, rrs []wire.Record) {
	if len(rrs) == 0 {
		return
	}
	fmt.Printf("%s: %d\n", label, len(rrs))
	for _, rr := range rrs {
		fmt.Printf("  %s %d %s %s %s\n", rr.Name, rr.TTL, classOf(rr), typeName(rr.Type), rdataString(rr))
	}
}

func printBudget(b *limits.Budget) {
	steps, names, records, bytes := b.Used()
	fmt.Printf("budget: steps=%d names=%d records=%d bytes=%d\n", steps, names, records, bytes)
}

func classOf(rr wire.Record) string {
	if rr.Type == wire.TypeOPT {
		return fmt.Sprintf("udpsize=%d", rr.Class)
	}
	return fmt.Sprintf("class=%d", rr.Class)
}

func typeName(t uint16) string {
	switch t {
	case wire.TypeA:
		return "A"
	case wire.TypeNS:
		return "NS"
	case wire.TypeCNAME:
		return "CNAME"
	case wire.TypeSOA:
		return "SOA"
	case wire.TypePTR:
		return "PTR"
	case wire.TypeMX:
		return "MX"
	case wire.TypeAAAA:
		return "AAAA"
	case wire.TypeDNAME:
		return "DNAME"
	case wire.TypeOPT:
		return "OPT"
	}
	return fmt.Sprintf("TYPE%d", t)
}

func rdataString(rr wire.Record) string {
	d := rr.Data
	switch rr.Type {
	case wire.TypeA:
		if len(d.Raw) == 4 {
			return net.IP(d.Raw).String()
		}
	case wire.TypeAAAA:
		if len(d.Raw) == 16 {
			return net.IP(d.Raw).String()
		}
	case wire.TypeMX:
		if len(d.Names) == 1 {
			return fmt.Sprintf("%d %s", d.Preference, d.Names[0])
		}
	case wire.TypeSOA:
		if len(d.Names) == 2 {
			return fmt.Sprintf("%s %s serial=%d refresh=%d retry=%d expire=%d minimum=%d",
				d.Names[0], d.Names[1], d.Serial, d.Refresh, d.Retry, d.Expire, d.Minimum)
		}
	case wire.TypeOPT:
		s := fmt.Sprintf("options=%d", len(d.Options))
		for _, o := range d.Options {
			s += fmt.Sprintf(" code=%d len=%d", o.Code, len(o.Data))
		}
		return s
	}
	if len(d.Names) > 0 {
		s := ""
		for _, n := range d.Names {
			s += n.String() + " "
		}
		return s
	}
	if len(d.Raw) > 0 {
		return "hex:" + hex.EncodeToString(d.Raw)
	}
	return ""
}
