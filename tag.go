package main

import (
	"flag"
	"fmt"
	"github.com/23jdd/mgit/object"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func runTag(args []string) error {
	fs := flag.NewFlagSet("tag", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	annotated := fs.Bool("a", false, "创建注解标签对象")
	message := fs.String("m", "", "标签说明；提供后会自动创建注解标签")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit tag [-a] [-m <标签说明>] <标签名> <对象哈希>")
		fmt.Fprintln(os.Stderr, "      mgit tag")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return listTags()
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return fmt.Errorf("tag 需要标签名和对象哈希")
	}

	name := fs.Arg(0)
	if err := validateRefPart(name); err != nil {
		return err
	}
	objectHash := fs.Arg(1)
	stored, err := object.ReadObject(objectHash)
	if err != nil {
		return err
	}

	refHash := objectHash
	if *annotated || strings.TrimSpace(*message) != "" {
		tag, err := object.NewTag(name, objectHash, stored.ObjectType, defaultSignature(), *message)
		if err != nil {
			return err
		}
		refHash, err = tag.Write()
		if err != nil {
			return err
		}
	}

	if err := writeRef("refs/tags/"+name, refHash); err != nil {
		return err
	}
	fmt.Println(refHash)
	return nil
}

func listTags() error {
	dir := filepath.Join(myGitDir, "refs", "tags")
	items, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取标签目录失败：%w", err)
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		names = append(names, item.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
