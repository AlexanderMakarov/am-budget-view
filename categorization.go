package main

import (
	"errors"
	"log"
)

// CategoryMatch represents the result of transaction categorization
type CategoryMatch struct {
	Name          string
	BySubstring   bool
	ByFromAccount bool
	ByToAccount   bool
}

// Categorization handles efficient categorization of transactions using
// pre-built trie and account mappings
type Categorization struct {
	// Indicates if all unknown transactions should be grouped.
	isGroupAllUnknownTransactions bool
	// Trie for efficient substring matching.
	trie *TrieNode
	// Mapping from 'from' accounts to group configurations.
	fromAccountToGroupConfig map[string]*GroupConfig
	// Mapping from 'to' accounts to group configurations.
	toAccountToGroupConfig map[string]*GroupConfig
}

// NewCategorization creates and initializes a new Categorization instance
func NewCategorization(config *Config) (*Categorization, error) {
	c := &Categorization{
		isGroupAllUnknownTransactions: config.GroupAllUnknownTransactions,
		trie:                          newTrieNode(),
		fromAccountToGroupConfig:      make(map[string]*GroupConfig),
		toAccountToGroupConfig:        make(map[string]*GroupConfig),
	}

	// Handle new Groups format.
	for groupName, group := range config.Groups {
		groupCopy := group
		groupCopy.Name = groupName

		// Add substrings. Check for duplicates.
		for _, substring := range group.Substrings {
			if err := c.trie.insert(substring, groupName, &groupCopy); err != nil {
				return nil, err
			}
		}

		// Add "from" accounts. Check for duplicates.
		for _, fromAccount := range group.FromAccounts {
			if duplicateGroup, ok := c.fromAccountToGroupConfig[fromAccount]; ok {
				return nil, errors.New(i18n.T(
					"wrong configuration: 'from' account a is duplicated in groups",
					"a", fromAccount,
					"group1", duplicateGroup.Name,
					"group2", groupName,
				))
			}
			c.fromAccountToGroupConfig[fromAccount] = &groupCopy
		}

		// Add "to" accounts. Check for duplicates.
		for _, toAccount := range group.ToAccounts {
			if duplicateGroup, ok := c.toAccountToGroupConfig[toAccount]; ok {
				return nil, errors.New(i18n.T(
					"wrong configuration: 'to' account a is duplicated in groups",
					"a", toAccount,
					"group1", duplicateGroup.Name,
					"group2", groupName,
				))
			}
			c.toAccountToGroupConfig[toAccount] = &groupCopy
		}
	}

	// Handle legacy GroupNamesToSubstrings
	for groupName, substrings := range config.GroupNamesToSubstrings {
		for _, substring := range substrings {

			groupConfig := &GroupConfig{
				Name: groupName,
			}
			if err := c.trie.insert(substring, groupName, groupConfig); err != nil {
				return nil, err
			}
		}
	}

	return c, nil
}

// CategorizeTransaction categorizes a single transaction using the pre-built trie
// and account mappings
func (c *Categorization) CategorizeTransaction(tr *Transaction) (*CategoryMatch, error) {
	// Validate transaction details.
	if tr.Details == "" {
		return nil, errors.New(i18n.T("empty details for transaction from f t", "f", tr.Source, "t", tr))
	}

	// First try to find matching group by accounts.
	if tr.FromAccount != "" {
		if groupConfig, ok := c.fromAccountToGroupConfig[tr.FromAccount]; ok {
			return &CategoryMatch{
				Name:          groupConfig.Name,
				ByFromAccount: true,
			}, nil
		}
	}
	if tr.ToAccount != "" {
		if groupConfig, ok := c.toAccountToGroupConfig[tr.ToAccount]; ok {
			return &CategoryMatch{
				Name:        groupConfig.Name,
				ByToAccount: true,
			}, nil
		}
	}

	// Try to find matching group in the trie
	if groupConfig := c.trie.findLongestMatchingGroup(tr.Details); groupConfig != nil {
		return &CategoryMatch{
			Name:        groupConfig.Name,
			BySubstring: true,
		}, nil
	}

	// Handle uncategorized case
	if c.isGroupAllUnknownTransactions {
		return &CategoryMatch{
			Name: UnknownGroupName,
		}, nil
	}
	return &CategoryMatch{
		Name: tr.Details,
	}, nil
}

// PrintUncategorizedTransactions prints transactions that couldn't be categorized
func (c *Categorization) PrintUncategorizedTransactions(transactions []Transaction) error {
	missedCnt := 0
	for _, tr := range transactions {
		if tr.Details == "" {
			return errors.New(i18n.T("empty details for transaction from f t", "f", tr.Source, "t", tr))
		}
		if groupConfig := c.trie.findLongestMatchingGroup(tr.Details); groupConfig == nil {
			log.Printf("Uncategorized transaction %+v", tr)
			missedCnt++
		}
	}

	lenTrans := len(transactions)
	log.Printf("Total %d uncategorized transactions from %d (%.2f%%)", missedCnt, lenTrans, float64(missedCnt)/float64(lenTrans)*100.00)
	return nil
}

// Trie Node structure.
type TrieNode struct {
	children    map[rune]*TrieNode
	isEnd       bool
	groupName   *string      // Pointer to the group name this node ends with
	groupConfig *GroupConfig // Store the group configuration
}

// Create a new Trie Node.
func newTrieNode() *TrieNode {
	return &TrieNode{
		children:  make(map[rune]*TrieNode),
		groupName: nil,
	}
}

// Insert a substring into the Trie with its group name. Returns error if duplicate exists.
func (t *TrieNode) insert(substring string, groupName string, config *GroupConfig) error {
	node := t
	for _, ch := range substring {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = newTrieNode()
		}
		node = node.children[ch]
	}

	// Check for duplicate at exact position.
	if node.isEnd {
		return errors.New(i18n.T(
			"wrong configuration: substring s is duplicated in groups",
			"s", substring,
			"group1", *node.groupName, // existing group name
			"group2", groupName, // new group name being inserted
		))
	}

	node.isEnd = true
	node.groupName = &groupName
	node.groupConfig = config
	return nil
}

// findLongestMatchingGroup finds the group with the longest matching substring in the trie
func (t *TrieNode) findLongestMatchingGroup(s string) *GroupConfig {
	runes := []rune(s)
	var bestMatch *GroupConfig
	var bestMatchLength int

	for i := 0; i < len(runes); i++ {
		node := t
		matchLength := 0
		for j := i; j < len(runes); j++ {
			ch := runes[j]
			if nextNode, ok := node.children[ch]; ok {
				node = nextNode
				matchLength++
				if node.isEnd && node.groupConfig != nil && matchLength > bestMatchLength {
					bestMatch = node.groupConfig
					bestMatchLength = matchLength
				}
			} else {
				break
			}
		}
	}
	return bestMatch
}
