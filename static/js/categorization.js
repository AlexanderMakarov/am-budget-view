let currentTransaction = null;
let accountsData = null;

// Wait for DOM to be fully loaded
document.addEventListener('DOMContentLoaded', function() {
    // Add form submit handler
    const newGroupForm = document.getElementById('newGroupForm');
    if (newGroupForm) {
        newGroupForm.addEventListener('submit', function(event) {
            event.preventDefault();
            
            const formData = new FormData(event.target);
            const data = {
                action: 'createGroup',
                groupName: formData.get('groupName'),
                substrings: formData.get('substrings'),
                fromAccounts: formData.get('fromAccounts'),
                toAccounts: formData.get('toAccounts')
            };
            
            fetch('/categorization', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(data)
            })
            .then(response => response.json())
            .then(updateTransactionsTable)
            .catch(error => console.error('Error:', error));
            
            closeNewGroupModal();
        });
    }

    // Add click outside handler
    document.addEventListener('click', function(event) {
        const popup = document.getElementById('categoryActionsPopup');
        if (!popup) return;
        
        const isClickInside = popup.contains(event.target);
        const isClickOnButton = event.target.classList.contains('action-button');
        
        if (!isClickInside && !isClickOnButton) {
            popup.style.display = 'none';
        }
    });

    // Initialize tooltip
    const tooltip = document.createElement('div');
    tooltip.className = 'tooltip';
    document.body.appendChild(tooltip);

    // Get accounts data from the template
    const accountsScript = document.getElementById('accountsData');
    if (accountsScript) {
        accountsData = JSON.parse(accountsScript.textContent);
    }

    // Add tooltip handlers for account cells
    document.querySelectorAll('td:nth-child(2), td:nth-child(3)').forEach(cell => {
        cell.addEventListener('mouseenter', function() {
            const accountName = cell.textContent.trim();
            const accountInfo = accountsData[accountName];
            
            if (!accountInfo) {
                tooltip.innerHTML = `<table><tr><td>${window.localizedStrings.unknownAccount}</td></tr></table>`;
            } else {
                let tooltipContent = '<table>';
                tooltipContent += `
                    <tr><td>${window.localizedStrings.type}:</td><td>${accountInfo.IsTransactionAccount ? window.localizedStrings.my : window.localizedStrings.unknown}</td></tr>
                    <tr><td>${window.localizedStrings.sourceType}:</td><td>${accountInfo.SourceType || ''}</td></tr>
                    <tr><td>${window.localizedStrings.source}:</td><td>${accountInfo.Source || ''}</td></tr>
                    <tr><td>${window.localizedStrings.from}:</td><td>${accountInfo.From || ''}</td></tr>
                    <tr><td>${window.localizedStrings.to}:</td><td>${accountInfo.To || ''}</td></tr>
                    <tr><td>${window.localizedStrings.occurencesInTransactions}:</td><td>${accountInfo.OccurencesInTransactions}</td></tr>
                `;
                tooltipContent += '</table>';
                tooltip.innerHTML = tooltipContent;
            }
            tooltip.style.display = 'block';
        });

        cell.addEventListener('mousemove', function(e) {
            tooltip.style.left = e.pageX + 10 + 'px';
            tooltip.style.top = e.pageY + 10 + 'px';
        });

        cell.addEventListener('mouseleave', function() {
            tooltip.style.display = 'none';
        });
    });
});

function showCategoryActions(button) {
    const modal = document.getElementById('categoryActionsModal');
    if (!modal) return;
    
    // Store the current transaction
    currentTransaction = button.closest('tr');
    
    // Pre-fill the substring value with transaction details
    const details = currentTransaction.querySelector('td:nth-child(5)').textContent;
    const substringInput = document.getElementById('categorySubstring');
    if (substringInput) {
        substringInput.value = details.trim();
    }
    
    // Use flex display to center the modal
    modal.style.display = 'flex';
}

function handleByMethodChange() {
    const byMethod = document.getElementById('categoryBy').value;
    const substringGroup = document.getElementById('substringGroup');
    
    if (byMethod === 'substring') {
        substringGroup.style.display = 'block';
    } else {
        substringGroup.style.display = 'none';
    }
}

function submitCategoryAction(event) {
    event.preventDefault();
    
    if (!currentTransaction) return;
    
    const groupName = document.getElementById('groupSelect').value;
    const byMethod = document.getElementById('categoryBy').value;
    
    let data = {
        action: 'addBy' + byMethod.charAt(0).toUpperCase() + byMethod.slice(1),
        groupName: groupName
    };
    
    if (byMethod === 'substring') {
        data.substring = document.getElementById('categorySubstring').value;
    } else if (byMethod === 'fromAccount') {
        data.fromAccount = currentTransaction.querySelector('td:nth-child(2)').textContent;
    } else if (byMethod === 'toAccount') {
        data.toAccount = currentTransaction.querySelector('td:nth-child(3)').textContent;
    }
    
    fetch('/categorization', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data)
    })
    .then(response => response.json())
    .then(updateTransactionsTable)
    .catch(error => console.error('Error:', error));
    
    closeCategoryModal();
}

function closeCategoryModal() {
    const modal = document.getElementById('categoryActionsModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

function openNewGroupModal() {
    const modal = document.getElementById('newGroupModal');
    if (modal) {
        // Use flex display to center the modal
        modal.style.display = 'flex';
    }
}

function closeNewGroupModal() {
    const modal = document.getElementById('newGroupModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

function updateTransactionsTable(transactions) {
    // Hide the popup
    const popup = document.getElementById('categoryActionsPopup');
    if (popup) {
        popup.style.display = 'none';
    }
    
    // Refresh the page to show updated transactions
    window.location.reload();
} 