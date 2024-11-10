package main

import (
	"errors"
	"log"
)

func PrintUncategorizedTransactions(transactions []Transaction, config *Config) error {
	trieRoot := newTrieNode()
	for _, substrings := range config.GroupNamesToSubstrings {
		for _, substring := range substrings {
			trieRoot.insert(substring)
		}
	}
	missedCnt := 0
	for _, tr := range transactions {
		if tr.Details == "" {
			return errors.New(i18n.T("empty details for transaction from f t", "f", tr.Source, "t", tr))
		}
		details := tr.Details
		if !trieRoot.searchSubstring(details) {
			log.Printf("Uncategorized transaction %+v", tr)
			missedCnt++
		}
	}
	lenTrans := len(transactions)
	log.Printf("Total %d uncategorized transactions from %d (%.2f%%)", missedCnt, lenTrans, float64(missedCnt)/float64(lenTrans)*100.00)
	return nil
}

// Trie Node structure
type TrieNode struct {
	children map[rune]*TrieNode
	isEnd    bool
}

// Create a new Trie Node
func newTrieNode() *TrieNode {
	return &TrieNode{children: make(map[rune]*TrieNode)}
}

// Insert a word into the Trie
func (t *TrieNode) insert(word string) {
	node := t
	for _, ch := range word {
		if _, ok := node.children[ch]; !ok {
			newNode := newTrieNode()
			node.children[ch] = newNode
			node = newNode
		} else {
			node = node.children[ch]
		}
	}
	node.isEnd = true
}

// searchSubstring checks if provided string contains any of the substrings in Trie.
func (t *TrieNode) searchSubstring(s string) bool {
	// Iterate over each rune in the string
	for i := 0; i < len([]rune(s)); i++ {
		// Start from the root of the Trie for each new starting position in the string
		node := t
		// Iterate over the string starting from the ith rune
		for j := i; j < len([]rune(s)); j++ {
			// Convert the jth character to a rune
			ch := []rune(s)[j]
			// Check if the current character exists in the Trie
			if nextNode, ok := node.children[ch]; ok {
				// If it exists, move to the next node in the Trie
				node = nextNode
				// If we've reached the end of a word in the Trie, return true
				if node.isEnd {
					return true
				}
			} else {
				// If the current character doesn't exist in the Trie, break the inner loop
				// and start a new search from the next starting position in the string
				break
			}
		}
	}
	// If no substring of the string is found in the Trie, return false
	return false
}
