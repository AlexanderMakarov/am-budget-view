package categorization

import (
	"errors"
	"log"

	"github.com/AlexanderMakarov/am-budget-view/internal/config"
	"github.com/AlexanderMakarov/am-budget-view/internal/i18n"
	"github.com/AlexanderMakarov/am-budget-view/internal/model"
)

const UnknownGroupName = "Unknown"

type groupConfigWithName struct {
	config.GroupConfig
	Name string
}

// Categorization handles efficient categorization of transactions using
// pre-built trie and accounts mappings.
type Categorization struct {
	isGroupAllUnknownTransactions bool
	trie                          *TrieNode
	fromAccountToGroupConfig      map[string]*groupConfigWithName
	toAccountToGroupConfig        map[string]*groupConfigWithName
}

// NewCategorization creates and initializes a new Categorization instance.
func NewCategorization(cfg *config.Config) (*Categorization, error) {
	c := &Categorization{
		isGroupAllUnknownTransactions: cfg.GroupAllUnknownTransactions,
		trie:                          newTrieNode(),
		fromAccountToGroupConfig:      make(map[string]*groupConfigWithName),
		toAccountToGroupConfig:        make(map[string]*groupConfigWithName),
	}

	for groupName, group := range cfg.Groups {
		groupCopy := &groupConfigWithName{
			GroupConfig: *group,
			Name:        groupName,
		}

		for _, substring := range group.Substrings {
			if err := c.trie.insert(substring, groupName, groupCopy); err != nil {
				return nil, err
			}
		}

		for _, fromAccount := range group.FromAccounts {
			if duplicateGroup, ok := c.fromAccountToGroupConfig[fromAccount]; ok {
				return nil, errors.New(i18n.T(
					"wrong configuration: 'from' account a is duplicated in groups",
					"a", fromAccount,
					"group1", duplicateGroup.Name,
					"group2", groupName,
				))
			}
			c.fromAccountToGroupConfig[fromAccount] = groupCopy
		}

		for _, toAccount := range group.ToAccounts {
			if duplicateGroup, ok := c.toAccountToGroupConfig[toAccount]; ok {
				return nil, errors.New(i18n.T(
					"wrong configuration: 'to' account a is duplicated in groups",
					"a", toAccount,
					"group1", duplicateGroup.Name,
					"group2", groupName,
				))
			}
			c.toAccountToGroupConfig[toAccount] = groupCopy
		}
	}

	return c, nil
}

// CategorizeTransaction categorizes a single transaction.
func (c *Categorization) CategorizeTransaction(tr *model.Transaction) (*model.CategoryMatch, bool, error) {
	if tr.Details == "" {
		return nil, false, errors.New(i18n.T("empty details for transaction from f t", "f", tr.Source, "t", tr))
	}

	if tr.FromAccount != "" {
		if groupConfig, ok := c.fromAccountToGroupConfig[tr.FromAccount]; ok {
			return &model.CategoryMatch{
				Name:      groupConfig.Name,
				RuleType:  model.RuleTypeFromAccount,
				RuleValue: tr.FromAccount,
			}, false, nil
		}
	}
	if tr.ToAccount != "" {
		if groupConfig, ok := c.toAccountToGroupConfig[tr.ToAccount]; ok {
			return &model.CategoryMatch{
				Name:      groupConfig.Name,
				RuleType:  model.RuleTypeToAccount,
				RuleValue: tr.ToAccount,
			}, false, nil
		}
	}

	groupConfig, matchedSubstring := c.trie.findLongestMatchingGroup(tr.Details)
	if groupConfig != nil {
		return &model.CategoryMatch{
			Name:      groupConfig.Name,
			RuleType:  model.RuleTypeSubstring,
			RuleValue: matchedSubstring,
		}, false, nil
	}

	if c.isGroupAllUnknownTransactions {
		return &model.CategoryMatch{
			Name: UnknownGroupName,
		}, true, nil
	} else {
		return &model.CategoryMatch{
			Name: tr.Details,
		}, true, nil
	}
}

// PrintUncategorizedTransactions prints transactions that couldn't be categorized
func (c *Categorization) PrintUncategorizedTransactions(transactions []model.Transaction) error {
	missedCnt := 0
	for _, tr := range transactions {
		if tr.Details == "" {
			return errors.New(i18n.T("empty details for transaction from f t", "f", tr.Source, "t", tr))
		}
		if groupConfig, _ := c.trie.findLongestMatchingGroup(tr.Details); groupConfig == nil {
			log.Printf("Uncategorized transaction %+v", tr)
			missedCnt++
		}
	}

	lenTrans := len(transactions)
	log.Printf("Total %d uncategorized transactions from %d (%.2f%%)", missedCnt, lenTrans, float64(missedCnt)/float64(lenTrans)*100.00)
	return nil
}

// GetUncategorizedTransactions returns transactions that couldn't be categorized
func (c *Categorization) GetUncategorizedTransactions(transactions []model.Transaction) []model.Transaction {
	var uncategorized []model.Transaction
	for _, tr := range transactions {
		if tr.Details == "" {
			continue
		}
		if groupConfig, _ := c.trie.findLongestMatchingGroup(tr.Details); groupConfig == nil {
			uncategorized = append(uncategorized, tr)
		}
	}
	return uncategorized
}

// TrieNode structure.
type TrieNode struct {
	children    map[rune]*TrieNode
	isEnd       bool
	groupName   *string
	groupConfig *groupConfigWithName
	substring   string
}

func newTrieNode() *TrieNode {
	return &TrieNode{
		children:  make(map[rune]*TrieNode),
		groupName: nil,
	}
}

func (t *TrieNode) insert(substring string, groupName string, cfg *groupConfigWithName) error {
	node := t
	for _, ch := range substring {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = newTrieNode()
		}
		node = node.children[ch]
	}

	if node.isEnd {
		return errors.New(i18n.T(
			"wrong configuration: substring s is duplicated in groups",
			"s", substring,
			"group1", *node.groupName,
			"group2", groupName,
		))
	}

	node.isEnd = true
	node.groupName = &groupName
	node.groupConfig = cfg
	node.substring = substring
	return nil
}

func (t *TrieNode) findLongestMatchingGroup(s string) (*groupConfigWithName, string) {
	runes := []rune(s)
	var bestMatch *groupConfigWithName
	var bestMatchSubstring string
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
					bestMatchSubstring = node.substring
					bestMatchLength = matchLength
				}
			} else {
				break
			}
		}
	}
	return bestMatch, bestMatchSubstring
}
