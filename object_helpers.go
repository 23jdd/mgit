package main

import (
	"fmt"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
)

func writeTreeFromIndex() (string, *object.Tree, error) {
	file, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return "", nil, err
	}
	return object.WriteTreeFromFiles(file.ToObjectFiles())
}

func printObject(stored *object.StoredObject) error {
	switch stored.ObjectType {
	case "blob", "commit", "tag":
		fmt.Print(string(stored.Payload))
	case "tree":
		tree, err := object.TreeFromStored(stored)
		if err != nil {
			return err
		}
		for _, entry := range tree.Entries {
			fmt.Printf("%s %s %s\t%s\n", entry.Mode, entry.ObjectType, entry.Hash, entry.Name)
		}
	default:
		fmt.Print(string(stored.Payload))
	}
	return nil
}

func requireObjectType(hash string, expected string) error {
	stored, err := object.ReadObject(hash)
	if err != nil {
		return err
	}
	if stored.ObjectType != expected {
		return fmt.Errorf("对象 %s 类型不匹配：期望 %s，实际 %s", hash, expected, stored.ObjectType)
	}
	return nil
}
