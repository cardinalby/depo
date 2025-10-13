package sql

import (
	"cmp"
	"fmt"
	"slices"
)

type MigrationCollection interface {
	GetEntries(afterVersion uint32) ([]MigrationEntry, error)
}

type MigrationCollectionBuilder interface {
	Add(version uint32, getStatements func() []string) MigrationCollectionBuilder
	GetCollection() MigrationCollection
}

type MigrationEntry struct {
	Version    uint32
	Statements []string
}

type migrationCollection struct {
	sorted []MigrationEntry
}

func (c migrationCollection) GetEntries(afterVersion uint32) ([]MigrationEntry, error) {
	if afterVersion == 0 {
		return c.sorted, nil
	}
	idx, found := slices.BinarySearchFunc(c.sorted, MigrationEntry{Version: afterVersion}, func(a, b MigrationEntry) int {
		return cmp.Compare(a.Version, b.Version)
	})
	if !found {
		return nil, fmt.Errorf("version %d not found", afterVersion)
	}
	if idx == len(c.sorted)-1 {
		return nil, nil
	}
	return c.sorted[idx+1:], nil
}

type migrationCollectionBuilder struct {
	versions map[uint32]func() []string
}

func NewMigrationCollectionBuilder() MigrationCollectionBuilder {
	return migrationCollectionBuilder{
		versions: make(map[uint32]func() []string),
	}
}

func (b migrationCollectionBuilder) Add(version uint32, getStatements func() []string) MigrationCollectionBuilder {
	if _, exists := b.versions[version]; exists {
		panic(fmt.Sprintf("migration version %d already exists", version))
	}
	b.versions[version] = getStatements
	return b
}

func (b migrationCollectionBuilder) GetCollection() MigrationCollection {
	entries := make([]MigrationEntry, 0, len(b.versions))
	for version, getStatements := range b.versions {
		entries = append(entries, MigrationEntry{
			Version:    version,
			Statements: getStatements(),
		})
	}
	slices.SortFunc(entries, func(a, b MigrationEntry) int {
		return cmp.Compare(a.Version, b.Version)
	})
	return migrationCollection{sorted: entries}
}
