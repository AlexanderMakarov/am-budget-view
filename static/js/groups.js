let currentEditGroup = null;
let currentEditType = null;
let currentRuleValue = null;

// Group name editing
document.querySelectorAll('.group-name').forEach(cell => {
    const display = cell.querySelector('.name-display');
    const input = cell.querySelector('.name-edit');
    
    display.addEventListener('click', () => {
        display.style.display = 'none';
        input.style.display = 'block';
        input.focus();
    });

    input.addEventListener('keydown', async (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            await applyNameChange(cell, input);
        } else if (e.key === 'Escape') {
            cancelNameEdit(cell);
        }
    });

    input.addEventListener('blur', () => {
        applyNameChange(cell, input);
    });
});

async function applyNameChange(cell, input) {
    const newName = input.value.trim();
    const originalName = cell.dataset.originalName;
    
    if (newName === originalName) {
        cancelNameEdit(cell);
        return;
    }

    if (newName === '') {
        alert(localizedStrings.groupNameEmpty);
        cancelNameEdit(cell);
        return;
    }

    try {
        const response = await fetch('/categorization', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                action: 'renameGroup',
                groupName: originalName,
                newGroupName: newName
            })
        });

        if (response.ok) {
            window.location.reload();
        } else {
            const error = await response.text();
            alert('Error: ' + error);
            cancelNameEdit(cell);
        }
    } catch (error) {
        console.error('Error:', error);
        alert('Error: ' + error.message);
        cancelNameEdit(cell);
    }
}

function cancelNameEdit(cell) {
    const display = cell.querySelector('.name-display');
    const input = cell.querySelector('.name-edit');
    display.style.display = 'block';
    input.style.display = 'none';
    input.value = cell.dataset.originalName;
}

// Rule editing
function editRule(ruleElement, groupName, ruleType) {
    currentEditGroup = groupName;
    currentEditType = ruleType;
    currentRuleValue = ruleElement.textContent;

    const modal = document.getElementById('ruleEditModal');
    const select = document.getElementById('ruleSelect');
    const valueInput = document.getElementById('ruleValue');

    // Clear and populate the select
    select.innerHTML = '';
    const rules = Array.from(document.querySelectorAll(`td[data-rule-type="${ruleType}"] .rule-item`))
        .filter(el => el.closest('tr').querySelector('.group-name').dataset.originalName === groupName)
        .map(el => el.textContent);
    
    rules.forEach(rule => {
        const option = document.createElement('option');
        option.value = rule;
        option.textContent = rule;
        select.appendChild(option);
    });

    // Set the current rule
    select.value = currentRuleValue;
    valueInput.value = currentRuleValue;

    modal.style.display = 'block';
}

function handleRuleSelect() {
    const select = document.getElementById('ruleSelect');
    const valueInput = document.getElementById('ruleValue');
    valueInput.value = select.value;
}

async function submitRuleEdit(event) {
    event.preventDefault();
    const oldValue = document.getElementById('ruleSelect').value;
    const newValue = document.getElementById('ruleValue').value.trim();

    if (newValue === '') {
        alert(localizedStrings.ruleValueEmpty);
        return;
    }

    try {
        // Get current group configuration
        const row = document.querySelector(`tr[data-original-name="${currentEditGroup}"]`);
        const fromAccounts = Array.from(row.querySelector('td[data-rule-type="fromAccounts"]').querySelectorAll('.rule-item'))
            .map(el => el.textContent);
        const toAccounts = Array.from(row.querySelector('td[data-rule-type="toAccounts"]').querySelectorAll('.rule-item'))
            .map(el => el.textContent);
        const substrings = Array.from(row.querySelector('td[data-rule-type="substrings"]').querySelectorAll('.rule-item'))
            .map(el => el.textContent);

        // Update the appropriate array
        const updateArray = (arr, oldVal, newVal) => {
            const index = arr.indexOf(oldVal);
            if (index !== -1) {
                arr[index] = newVal;
            }
            return arr;
        };

        let updatedConfig = {
            fromAccounts: fromAccounts,
            toAccounts: toAccounts,
            substrings: substrings
        };

        switch (currentEditType) {
            case 'fromAccounts':
                updatedConfig.fromAccounts = updateArray(fromAccounts, oldValue, newValue);
                break;
            case 'toAccounts':
                updatedConfig.toAccounts = updateArray(toAccounts, oldValue, newValue);
                break;
            case 'substrings':
                updatedConfig.substrings = updateArray(substrings, oldValue, newValue);
                break;
        }

        const response = await fetch('/categorization', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                action: 'upsertGroup',
                groupName: currentEditGroup,
                fromAccounts: updatedConfig.fromAccounts,
                toAccounts: updatedConfig.toAccounts,
                substrings: updatedConfig.substrings
            })
        });

        if (response.ok) {
            window.location.reload();
        } else {
            alert('Error: ' + await response.text());
        }
    } catch (error) {
        console.error('Error:', error);
        alert('Error: ' + error.message);
    }
}

async function deleteRule() {
    const ruleValue = document.getElementById('ruleSelect').value;
    
    if (!confirm(localizedStrings.confirmDeleteRule)) {
        return;
    }

    try {
        const row = document.querySelector(`td.group-name[data-original-name="${currentEditGroup}"]`).closest('tr');
        if (!row) {
            throw new Error('Could not find the group row');
        }

        const fromAccounts = Array.from(row.querySelector('td[data-rule-type="fromAccounts"]').querySelectorAll('.rule-item'))
            .map(el => el.textContent);
        const toAccounts = Array.from(row.querySelector('td[data-rule-type="toAccounts"]').querySelectorAll('.rule-item'))
            .map(el => el.textContent);
        const substrings = Array.from(row.querySelector('td[data-rule-type="substrings"]').querySelectorAll('.rule-item'))
            .map(el => el.textContent);

        // Remove the rule from the appropriate array
        const removeFromArray = (arr, val) => arr.filter(item => item !== val);

        let updatedConfig = {
            fromAccounts: fromAccounts,
            toAccounts: toAccounts,
            substrings: substrings
        };

        switch (currentEditType) {
            case 'fromAccounts':
                updatedConfig.fromAccounts = removeFromArray(fromAccounts, ruleValue);
                break;
            case 'toAccounts':
                updatedConfig.toAccounts = removeFromArray(toAccounts, ruleValue);
                break;
            case 'substrings':
                updatedConfig.substrings = removeFromArray(substrings, ruleValue);
                break;
        }

        const response = await fetch('/categorization', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                action: 'upsertGroup',
                groupName: currentEditGroup,
                fromAccounts: updatedConfig.fromAccounts,
                toAccounts: updatedConfig.toAccounts,
                substrings: updatedConfig.substrings
            })
        });

        if (response.ok) {
            window.location.reload();
        } else {
            alert('Error: ' + await response.text());
        }
    } catch (error) {
        console.error('Error:', error);
        alert('Error: ' + error.message);
    }
}

function closeRuleModal() {
    document.getElementById('ruleEditModal').style.display = 'none';
}

// Close modal if clicking outside
window.onclick = function(event) {
    const modal = document.getElementById('ruleEditModal');
    if (event.target === modal) {
        closeRuleModal();
    }
}

async function deleteGroup(groupName) {
    if (!confirm(localizedStrings.confirmDeleteGroup)) {
        return;
    }

    try {
        const response = await fetch('/categorization', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                action: 'deleteGroup',
                groupName: groupName
            })
        });

        if (response.ok) {
            window.location.reload();
        } else {
            alert('Error: ' + await response.text());
        }
    } catch (error) {
        console.error('Error:', error);
        alert('Error: ' + error.message);
    }
} 