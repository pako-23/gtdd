package algorithms_test

import (
	"fmt"
	"testing"

	"github.com/pako-23/gtdd/internal/algorithms"
	"github.com/pako-23/gtdd/internal/runner"
	"gotest.tools/v3/assert"
)

type mockRunner struct {
	dependencyMap map[string][][]string
	id            string
}

func newMockRunnerBuilder(id string, options ...runner.RunnerOption[*mockRunner]) (*mockRunner, error) {
	runner := &mockRunner{dependencyMap: map[string][][]string{}, id: id}

	for _, option := range options {
		if err := option(runner); err != nil {
			return nil, err
		}
	}

	return runner, nil
}

func withDependencyMap(dependecyMap map[string][][]string) func(*mockRunner) error {
	return func(runner *mockRunner) error {
		runner.dependencyMap = dependecyMap
		return nil
	}
}

func (m *mockRunner) ResetApplication() error {
	return nil
}

func (m *mockRunner) Delete() error {
	return nil
}

func (m *mockRunner) Id() string {
	return m.id
}

func (m *mockRunner) Run(tests []string) ([]bool, error) {
	results := make([]bool, len(tests))

	for i := range tests {
		deps, ok := m.dependencyMap[tests[i]]
		if !ok || len(deps) == 0 {
			results[i] = true
			continue
		}

		for _, dep := range deps {
			j, k := 0, 0
			for j < len(dep) && k < i {
				if dep[j] == tests[k] {
					j++
				}
				k++
			}

			if j == len(dep) {
				results[i] = true
				break
			}
		}

		if !results[i] {
			break
		}
	}

	return results, nil
}

func testNoDependencies(t *testing.T, algo algorithms.DependencyDetector) {
	t.Parallel()

	var tests = [][]string{
		{"test1", "test2", "test3"},
		{"test1"},
		{},
	}

	for _, test := range tests {
		runner, _ := runner.NewRunnerSet[*mockRunner](5, newMockRunnerBuilder)
		got, err := algo(test, runner)
		expected := algorithms.NewDependencyGraph(test)

		assert.NilError(t, err)
		assert.Check(t, got.Equal(expected),
			fmt.Sprintf("expected graph %v, but got %v", got, expected))
	}
}

func testExistingDependencies(t *testing.T, algo algorithms.DependencyDetector) {
	t.Parallel()

	var tests = []struct {
		testsuite    []string
		dependencies map[string][][]string
		expected     algorithms.DependencyGraph
	}{
		{
			testsuite: []string{"test1", "test2", "test3"},
			dependencies: map[string][][]string{
				"test2": {{"test1"}},
				"test3": {{"test1", "test2"}},
			},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"test1": {},
				"test2": {"test1": {}},
				"test3": {"test2": {}},
			}),
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5"},
			dependencies: map[string][][]string{
				"test3": {{"test1", "test2"}},
				"test5": {{"test1", "test2", "test3"}},
			},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"test1": {},
				"test2": {},
				"test3": {"test1": {}, "test2": {}},
				"test4": {},
				"test5": {"test3": {}},
			}),
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5"},
			dependencies: map[string][][]string{
				"test5": {{"test1", "test2", "test3", "test4"}},
			},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"test1": {},
				"test2": {},
				"test3": {},
				"test4": {},
				"test5": {"test1": {}, "test2": {}, "test3": {}, "test4": {}},
			}),
		},
		{
			testsuite: []string{
				"tests.AddressBookAddAddressBookTest",
				"tests.AddressBookSearchAddressBookNameTest",
				"tests.AddressBookSearchAddressBookEmailTest",
				"tests.AddressBookAddGroupTest",
				"tests.AddressBookAssignToGroupTest",
				"tests.AddressBookSearchByGroupTest",
				"tests.AddressBookCheckBirthdayInfoTest",
				"tests.AddressBookCheckAddressBookTest",
				"tests.AddressBookPrintAddressBookTest",
				"tests.AddressBookEditAddressBookTest",
				"tests.AddressBookEditGroupTest",
				"tests.AddressBookRemoveFromGroupTest",
				"tests.AddressBookRemoveGroupTest",
				"tests.AddressBookRemoveAddressBookTest",
				"tests.AddressBookAddMultipleAddressBookTest",
				"tests.AddressBookSearchMultipleAddressBookNameTest",
				"tests.AddressBookAddMultipleGroupsTest",
				"tests.AddressBookAssignToMultipleGroupsTest",
				"tests.AddressBookSearchByMultipleGroupsTest",
				"tests.AddressBookCheckMultipleBirthdaysInfoTest",
				"tests.AddressBookCheckMultipleAddressBookTest",
				"tests.AddressBookPrintMultipleAddressBookTest",
				"tests.AddressBookEditMultipleAddressBookTest",
				"tests.AddressBookEditMultipleGroupsTest",
				"tests.AddressBookRemoveFromMultipleGroupsTest",
				"tests.AddressBookRemoveMultipleGroupsTest",
				"tests.AddressBookRemoveMultipleAddressBookTest",
			},
			dependencies: map[string][][]string{
				"tests.AddressBookAddMultipleAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
					},
				},
				"tests.AddressBookAssignToGroupTest": {
					{"tests.AddressBookAddAddressBookTest"},
				},
				"tests.AddressBookAssignToMultipleGroupsTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
						"tests.AddressBookAddMultipleGroupsTest",
					},
				},
				"tests.AddressBookCheckAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookCheckBirthdayInfoTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookCheckMultipleAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
					},
				},
				"tests.AddressBookCheckMultipleBirthdaysInfoTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
					},
				},
				"tests.AddressBookEditAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookEditGroupTest": {
					{
						"tests.AddressBookAddGroupTest",
					},
				},
				"tests.AddressBookEditMultipleAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
					},
				},
				"tests.AddressBookEditMultipleGroupsTest": {
					{
						"tests.AddressBookAddGroupTest",
						"tests.AddressBookRemoveGroupTest",
						"tests.AddressBookAddMultipleGroupsTest",
					},
				},
				"tests.AddressBookPrintAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookPrintMultipleAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
					},
				},
				"tests.AddressBookRemoveAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookRemoveFromGroupTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookAddGroupTest",
						"tests.AddressBookAssignToGroupTest",
						"tests.AddressBookEditGroupTest",
					},
				},
				"tests.AddressBookRemoveFromMultipleGroupsTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookAddGroupTest",
						"tests.AddressBookRemoveGroupTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
						"tests.AddressBookAddMultipleGroupsTest",
						"tests.AddressBookAssignToMultipleGroupsTest",
						"tests.AddressBookEditMultipleGroupsTest",
					},
				},
				"tests.AddressBookRemoveGroupTest": {
					{
						"tests.AddressBookAddGroupTest",
					},
				},
				"tests.AddressBookRemoveMultipleAddressBookTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
					},
				},
				"tests.AddressBookRemoveMultipleGroupsTest": {
					{
						"tests.AddressBookAddGroupTest",
						"tests.AddressBookRemoveGroupTest",
						"tests.AddressBookAddMultipleGroupsTest",
					},
				},
				"tests.AddressBookSearchAddressBookEmailTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookSearchAddressBookNameTest": {
					{
						"tests.AddressBookAddAddressBookTest",
					},
				},
				"tests.AddressBookSearchByGroupTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookAddGroupTest",
						"tests.AddressBookAssignToGroupTest",
					},
				},
				"tests.AddressBookSearchByMultipleGroupsTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
						"tests.AddressBookAddMultipleGroupsTest",
						"tests.AddressBookAssignToMultipleGroupsTest",
					},
				},
				"tests.AddressBookSearchMultipleAddressBookNameTest": {
					{
						"tests.AddressBookAddAddressBookTest",
						"tests.AddressBookRemoveAddressBookTest",
						"tests.AddressBookAddMultipleAddressBookTest",
					},
				},
			},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"tests.AddressBookAddAddressBookTest": {},
				"tests.AddressBookAddGroupTest":       {},
				"tests.AddressBookAddMultipleAddressBookTest": {
					"tests.AddressBookRemoveAddressBookTest": {},
				},
				"tests.AddressBookAddMultipleGroupsTest": {},
				"tests.AddressBookAssignToGroupTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookAssignToMultipleGroupsTest": {
					"tests.AddressBookAddMultipleGroupsTest":      {},
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
				"tests.AddressBookCheckAddressBookTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookCheckBirthdayInfoTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookCheckMultipleAddressBookTest": {
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
				"tests.AddressBookCheckMultipleBirthdaysInfoTest": {
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
				"tests.AddressBookEditAddressBookTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookEditGroupTest": {
					"tests.AddressBookAddGroupTest": {},
				},
				"tests.AddressBookEditMultipleAddressBookTest": {
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
				"tests.AddressBookEditMultipleGroupsTest": {
					"tests.AddressBookAddMultipleGroupsTest": {},
					"tests.AddressBookRemoveGroupTest":       {},
				},
				"tests.AddressBookPrintAddressBookTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookPrintMultipleAddressBookTest": {
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
				"tests.AddressBookRemoveAddressBookTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookRemoveFromGroupTest": {
					"tests.AddressBookEditGroupTest":     {},
					"tests.AddressBookAssignToGroupTest": {},
				},
				"tests.AddressBookRemoveFromMultipleGroupsTest": {
					"tests.AddressBookEditMultipleGroupsTest":     {},
					"tests.AddressBookAssignToMultipleGroupsTest": {},
				},
				"tests.AddressBookRemoveGroupTest": {
					"tests.AddressBookAddGroupTest": {},
				},
				"tests.AddressBookRemoveMultipleAddressBookTest": {
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
				"tests.AddressBookRemoveMultipleGroupsTest": {
					"tests.AddressBookRemoveGroupTest":       {},
					"tests.AddressBookAddMultipleGroupsTest": {},
				},
				"tests.AddressBookSearchAddressBookEmailTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookSearchAddressBookNameTest": {
					"tests.AddressBookAddAddressBookTest": {},
				},
				"tests.AddressBookSearchByGroupTest": {
					"tests.AddressBookAssignToGroupTest": {},
					"tests.AddressBookAddGroupTest":      {},
				},
				"tests.AddressBookSearchByMultipleGroupsTest": {
					"tests.AddressBookAssignToMultipleGroupsTest": {},
				},
				"tests.AddressBookSearchMultipleAddressBookNameTest": {
					"tests.AddressBookAddMultipleAddressBookTest": {},
				},
			}),
		},
		{
			testsuite: []string{
				"PasswordManagerAddEntryTest",
				"PasswordManagerSearchEntryByNameTest",
				"PasswordManagerSearchEntryByUsernameTest",
				"PasswordManagerSearchEntryByUrlTest",
				"PasswordManagerSearchEntryByTagsTest",
				"PasswordManagerSearchEntryByCommentTest",
				"PasswordManagerSearchEntryByTagListTest",
				"PasswordManagerEditEntryTest",
				"PasswordManagerSearchTagsTest",
				"PasswordManagerRemoveTagsTest",
				"PasswordManagerCheckEntryTagsRemovedTest",
				"PasswordManagerRemoveEntryTest",
				"PasswordManagerSearchEntryNegativeTest",
				"PasswordManagerSearchTagNegativeTest",
				"PasswordManagerAddTagTest",
				"PasswordManagerEditTagTest",
				"PasswordManagerRemoveTagTest",
				"PasswordManagerAssignTagToEntryTest",
				"PasswordManagerAddMultipleEntriesTest",
				"PasswordManagerSearchMultipleEntriesTest",
				"PasswordManagerCheckUsedTagsTest",
				"PasswordManagerSearchAndRemoveMultipleTagsTest",
				"PasswordManagerRemoveMultipleEntriesTest",
			},
			dependencies: map[string][][]string{
				"PasswordManagerSearchEntryByNameTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerSearchEntryByTagsTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerSearchTagsTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerAssignTagToEntryTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
					},
				},
				"PasswordManagerCheckUsedTagsTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
						"PasswordManagerRemoveEntryTest",
						"PasswordManagerAssignTagToEntryTest",
						"PasswordManagerAddMultipleEntriesTest",
					},
				},
				"PasswordManagerSearchAndRemoveMultipleTagsTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
						"PasswordManagerRemoveEntryTest",
						"PasswordManagerAssignTagToEntryTest",
						"PasswordManagerAddMultipleEntriesTest",
					},
				},
				"PasswordManagerSearchEntryByCommentTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerEditEntryTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerRemoveTagsTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerRemoveMultipleEntriesTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveEntryTest",
						"PasswordManagerAddMultipleEntriesTest",
					},
				},
				"PasswordManagerSearchEntryByUsernameTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerCheckEntryTagsRemovedTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
					},
				},
				"PasswordManagerAddTagTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
					},
				},
				"PasswordManagerAddMultipleEntriesTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveEntryTest",
					},
				},
				"PasswordManagerSearchEntryByUrlTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerSearchEntryByTagListTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerRemoveEntryTest": {
					{"PasswordManagerAddEntryTest"},
				},
				"PasswordManagerEditTagTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
						"PasswordManagerAddTagTest",
					},
				},
				"PasswordManagerRemoveTagTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveTagsTest",
						"PasswordManagerAddTagTest",
					},
				},
				"PasswordManagerSearchMultipleEntriesTest": {
					{
						"PasswordManagerAddEntryTest",
						"PasswordManagerRemoveEntryTest",
						"PasswordManagerAddMultipleEntriesTest",
					},
				},
			},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"PasswordManagerAddEntryTest": {},
				"PasswordManagerAddMultipleEntriesTest": {
					"PasswordManagerRemoveEntryTest": {},
				},
				"PasswordManagerAddTagTest": {
					"PasswordManagerRemoveTagsTest": {},
				},
				"PasswordManagerAssignTagToEntryTest": {
					"PasswordManagerRemoveTagsTest": {},
				},
				"PasswordManagerCheckEntryTagsRemovedTest": {
					"PasswordManagerRemoveTagsTest": {},
				},
				"PasswordManagerCheckUsedTagsTest": {
					"PasswordManagerAddMultipleEntriesTest": {},
					"PasswordManagerAssignTagToEntryTest":   {},
				},
				"PasswordManagerEditEntryTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerEditTagTest": {
					"PasswordManagerAddTagTest": {},
				},
				"PasswordManagerRemoveEntryTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerRemoveMultipleEntriesTest": {
					"PasswordManagerAddMultipleEntriesTest": {},
				},
				"PasswordManagerRemoveTagTest": {
					"PasswordManagerAddTagTest": {},
				},
				"PasswordManagerRemoveTagsTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchAndRemoveMultipleTagsTest": {
					"PasswordManagerAddMultipleEntriesTest": {},
					"PasswordManagerAssignTagToEntryTest":   {},
				},
				"PasswordManagerSearchEntryByCommentTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchEntryByNameTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchEntryByTagListTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchEntryByTagsTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchEntryByUrlTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchEntryByUsernameTest": {
					"PasswordManagerAddEntryTest": {},
				},
				"PasswordManagerSearchEntryNegativeTest": {},
				"PasswordManagerSearchMultipleEntriesTest": {
					"PasswordManagerAddMultipleEntriesTest": {},
				},
				"PasswordManagerSearchTagNegativeTest": {},
				"PasswordManagerSearchTagsTest": {
					"PasswordManagerAddEntryTest": {},
				}}),
		},
		{
			testsuite: []string{
				"AddUserTest",
				"LoginUserTest",
				"AddExistingUserFailsTest",
				"AddProductTest",
				"AddNewProdToCartTest",
				"SearchProductTest",
				"AddReviewTest",
				"SeeReviewTest",
				"AddDiscountCodeAmountTest",
				"AddDiscountCodePercentTest",
				"UseDiscountCodeAmountTest",
				"UseDiscountCodePercentTest",
				"AddProductTagTest",
				"SearchProductTagTest",
				"AddMenuTest",
				"OpenMenuTest",
				"DeleteDiscountCodeAmountTest",
				"DeleteDiscountCodePercentTest",
				"DeletedDiscountCodeFailsAmountTest",
				"DeletedDiscountCodeFailsPercentTest",
				"DeleteUserTest",
				"LoginDeletedUserFailsTest",
				"DeleteReviewTest",
				"DeleteProductTagTest",
				"SearchDeletedProductTagFailsTest",
				"DeleteProductTest",
				"SearchDeletedProductFailsTest",
			},
			dependencies: map[string][][]string{
				"SearchProductTest":             {{"AddProductTest"}},
				"DeleteDiscountCodePercentTest": {{"AddDiscountCodePercentTest"}},
				"DeletedDiscountCodeFailsAmountTest": {
					{
						"AddDiscountCodeAmountTest",
						"DeleteDiscountCodeAmountTest",
					},
				},
				"LoginDeletedUserFailsTest": {
					{
						"AddUserTest",
						"DeleteUserTest",
					},
				},
				"DeletedDiscountCodeFailsPercentTest": {
					{
						"AddDiscountCodePercentTest",
						"DeleteDiscountCodePercentTest",
					},
				},
				"DeleteUserTest": {{"AddUserTest"}},
				"LoginUserTest":  {{"AddUserTest"}},
				"SeeReviewTest": {
					{
						"AddProductTest",
						"AddReviewTest",
					},
				},
				"OpenMenuTest": {
					{
						"AddProductTest",
						"AddProductTagTest",
						"AddMenuTest",
					},
				},
				"DeleteDiscountCodeAmountTest": {{"AddDiscountCodeAmountTest"}},
				"AddNewProdToCartTest":         {{"AddProductTest"}},
				"SearchProductTagTest": {
					{
						"AddProductTest",
						"AddProductTagTest",
					},
				},
				"DeleteProductTagTest": {
					{
						"AddProductTest",
						"AddProductTagTest",
					},
				},
				"SearchDeletedProductFailsTest": {{"DeleteProductTest"}},
				"AddProductTagTest":             {{"AddProductTest"}},
				"SearchDeletedProductTagFailsTest": {
					{
						"AddProductTest",
						"AddProductTagTest",
						"DeleteProductTagTest",
					},
				},
				"AddExistingUserFailsTest":  {{"AddUserTest"}},
				"AddReviewTest":             {{"AddProductTest"}},
				"UseDiscountCodeAmountTest": {{"AddDiscountCodeAmountTest"}},
				"UseDiscountCodePercentTest": {
					{
						"AddProductTest",
						"AddDiscountCodePercentTest",
					},
				},
			},
			expected: algorithms.DependencyGraph(map[string]map[string]struct{}{
				"AddDiscountCodeAmountTest":  {},
				"AddDiscountCodePercentTest": {},
				"AddExistingUserFailsTest": {
					"AddUserTest": {},
				},
				"AddMenuTest": {},
				"AddNewProdToCartTest": {
					"AddProductTest": {},
				},
				"AddProductTagTest": {
					"AddProductTest": {},
				},
				"AddProductTest": {},
				"AddReviewTest": {
					"AddProductTest": {},
				},
				"AddUserTest": {},
				"DeleteDiscountCodeAmountTest": {
					"AddDiscountCodeAmountTest": {},
				},
				"DeleteDiscountCodePercentTest": {
					"AddDiscountCodePercentTest": {},
				},
				"DeleteProductTagTest": {
					"AddProductTagTest": {},
				},
				"DeleteProductTest": {},
				"DeleteReviewTest":  {},
				"DeleteUserTest": {
					"AddUserTest": {},
				},
				"DeletedDiscountCodeFailsAmountTest": {
					"DeleteDiscountCodeAmountTest": {},
				},
				"DeletedDiscountCodeFailsPercentTest": {
					"DeleteDiscountCodePercentTest": {},
				},
				"LoginDeletedUserFailsTest": {
					"DeleteUserTest": {},
				},
				"LoginUserTest": {
					"AddUserTest": {},
				},
				"OpenMenuTest": {
					"AddMenuTest":       {},
					"AddProductTagTest": {},
				},
				"SearchDeletedProductFailsTest": {
					"DeleteProductTest": {},
				},
				"SearchDeletedProductTagFailsTest": {
					"DeleteProductTagTest": {},
				},
				"SearchProductTagTest": {
					"AddProductTagTest": {},
				},
				"SearchProductTest": {
					"AddProductTest": {},
				},
				"SeeReviewTest": {
					"AddReviewTest": {},
				},
				"UseDiscountCodeAmountTest": {
					"AddDiscountCodeAmountTest": {},
				},
				"UseDiscountCodePercentTest": {
					"AddDiscountCodePercentTest": {},
					"AddProductTest":             {},
				},
			}),
		},
	}

	for _, test := range tests {
		runner, _ := runner.NewRunnerSet[*mockRunner](12,
			newMockRunnerBuilder,
			withDependencyMap(test.dependencies))
		got, err := algo(test.testsuite, runner)

		assert.NilError(t, err)
		assert.Check(t, got.Equal(test.expected),
			fmt.Sprintf("expected graph %v, but got %v", test.expected, got))
	}
}

func testOrDependencies(t *testing.T, algo algorithms.DependencyDetector) {
	t.Parallel()

	var tests = []struct {
		testsuite    []string
		dependencies map[string][][]string
		expected     []algorithms.DependencyGraph
	}{
		{
			testsuite: []string{"test1", "test2", "test3"},
			dependencies: map[string][][]string{
				"test3": {{"test1"}, {"test2"}},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {},
					"test3": {"test1": {}},
				}),
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {},
					"test3": {"test2": {}},
				}),
			},
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5"},
			dependencies: map[string][][]string{
				"test3": {{"test2"}},
				"test4": {{"test2", "test3"}},
				"test5": {
					{"test2", "test3", "test4"},
					{"test1"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {},
					"test3": {"test2": {}},
					"test4": {"test3": {}},
					"test5": {"test1": {}},
				}),
			},
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5"},
			dependencies: map[string][][]string{
				"test5": {
					{"test2", "test4"},
					{"test1", "test3"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {},
					"test3": {},
					"test4": {},
					"test5": {"test1": {}, "test3": {}},
				}),
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {},
					"test3": {},
					"test4": {},
					"test5": {"test2": {}, "test4": {}},
				}),
			},
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5", "test6"},
			dependencies: map[string][][]string{
				"test3": {{"test2"}},
				"test4": {{"test2", "test3"}},
				"test5": {
					{"test2", "test3", "test4"},
					{"test1"},
				},
				"test6": {
					{"test2", "test3", "test4", "test5"},
					{"test1", "test5"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {},
					"test3": {"test2": {}},
					"test4": {"test3": {}},
					"test5": {"test1": {}},
					"test6": {"test5": {}},
				}),
			},
		},
	}

	for _, test := range tests {
		runner, _ := runner.NewRunnerSet[*mockRunner](5,
			newMockRunnerBuilder,
			withDependencyMap(test.dependencies))
		got, err := algo(test.testsuite, runner)

		assert.NilError(t, err)
		found := false

		for _, expectedDeps := range test.expected {
			if expectedDeps.Equal(got) {
				found = true
				break
			}
		}

		assert.Check(t, found,
			fmt.Sprintf("expected one of the following graphs %v, but got %v", test.expected, got))
	}
}

func testMinLenOrDependencies(t *testing.T, algo algorithms.DependencyDetector) {
	t.Parallel()

	var tests = []struct {
		testsuite    []string
		dependencies map[string][][]string
		expected     []algorithms.DependencyGraph
	}{
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5"},
			dependencies: map[string][][]string{
				"test2": {{"test1"}},
				"test3": {{"test1", "test2"}},
				"test5": {
					{"test1", "test2", "test3"},
					{"test4"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {"test1": {}},
					"test3": {"test2": {}},
					"test4": {},
					"test5": {"test4": {}},
				}),
			},
		},
		{
			testsuite: []string{"test1", "test2", "test3", "test4", "test5", "test6"},
			dependencies: map[string][][]string{
				"test2": {{"test1"}},
				"test3": {{"test1", "test2"}},
				"test5": {
					{"test1", "test2", "test3"},
					{"test4"},
				},
				"test6": {
					{"test1", "test2", "test3", "test5"},
					{"test4", "test5"},
				},
			},
			expected: []algorithms.DependencyGraph{
				algorithms.DependencyGraph(map[string]map[string]struct{}{
					"test1": {},
					"test2": {"test1": {}},
					"test3": {"test2": {}},
					"test4": {},
					"test5": {"test4": {}},
					"test6": {"test5": {}},
				}),
			},
		},
	}

	for _, test := range tests {
		runner, _ := runner.NewRunnerSet[*mockRunner](5,
			newMockRunnerBuilder,
			withDependencyMap(test.dependencies))
		got, err := algo(test.testsuite, runner)

		assert.NilError(t, err)
		found := false

		for _, expectedDeps := range test.expected {
			if expectedDeps.Equal(got) {
				found = true
				break
			}
		}

		assert.Check(t, found,
			fmt.Sprintf("expected one of the following graphs %v, but got %v", test.expected, got))
	}
}
