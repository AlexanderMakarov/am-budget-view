let currentTransaction = null;
let accountsData = null;
let groupsData = null;

// Wait for DOM to be fully loaded
document.addEventListener("DOMContentLoaded", function () {
    // Get accounts data from the template
    const accountsScript = document.getElementById("accountsData");
    if (accountsScript) {
        accountsData = JSON.parse(accountsScript.textContent);
    }

    // Get groups data from the template
    const groupsScript = document.getElementById("groupsData");
    if (groupsScript) {
        try {
            groupsData = JSON.parse(groupsScript.textContent);
        } catch (error) {
            console.error("Error parsing groups data:", error);
            groupsData = {};
        }
    }

    // Initialize group select options
    const groupSelect = document.getElementById("groupSelect");
    if (groupSelect && groupsData) {
        Object.keys(groupsData).sort().forEach(groupName => {
            const option = document.createElement("option");
            option.value = groupName;
            option.textContent = groupName;
            groupSelect.appendChild(option);
        });
    }

    // Initialize tooltip
    const tooltip = document.createElement("div");
    tooltip.className = "tooltip";
    document.body.appendChild(tooltip);

    // Add tooltip handlers for account cells
    document.querySelectorAll("td:nth-child(2), td:nth-child(3)").forEach((cell) => {
        cell.addEventListener("mouseenter", function () {
            const accountName = cell.textContent.trim();
            const accountInfo = accountsData[accountName];

            if (!accountInfo) {
                tooltip.innerHTML = `<table><tr><td>${window.localizedStrings.unknownAccount}</td></tr></table>`;
            } else {
                let tooltipContent = "<table>";
                tooltipContent += `
                    <tr><td>${window.localizedStrings.type}:</td><td>${
                    accountInfo.IsTransactionAccount ? window.localizedStrings.my : window.localizedStrings.unknown
                }</td></tr>
                    <tr><td>${window.localizedStrings.sourceType}:</td><td>${accountInfo.SourceType || ""}</td></tr>
                    <tr><td>${window.localizedStrings.source}:</td><td>${accountInfo.Source || ""}</td></tr>
                    <tr><td>${window.localizedStrings.from}:</td><td>${accountInfo.From || ""}</td></tr>
                    <tr><td>${window.localizedStrings.to}:</td><td>${accountInfo.To || ""}</td></tr>
                    <tr><td>${window.localizedStrings.occurencesInTransactions}:</td><td>${
                    accountInfo.OccurencesInTransactions
                }</td></tr>
                `;
                tooltipContent += "</table>";
                tooltip.innerHTML = tooltipContent;
            }
            tooltip.style.display = "block";
        });

        cell.addEventListener("mousemove", function (e) {
            tooltip.style.left = e.pageX + 10 + "px";
            tooltip.style.top = e.pageY + 10 + "px";
        });

        cell.addEventListener("mouseleave", function () {
            tooltip.style.display = "none";
        });
    });
});

function submitNewGroup(event) {
    event.preventDefault();

    const formData = new FormData(event.target);
    const groupName = formData.get("groupName");

    // Check if group already exists
    const groupSelect = document.getElementById("groupSelect");
    const existingGroups = Array.from(groupSelect.options).map((option) => option.value);

    if (existingGroups.includes(groupName)) {
        alert(window.localizedStrings.groupAlreadyExists || "Group with this name already exists");
        return;
    }

    const data = {
        action: "upsertGroup",
        groupName: groupName,
    };

    fetch("/categorization", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
    })
        .then(() => window.location.reload())
        .catch((error) => console.error("Error:", error));

    closeNewGroupModal();
}

function getExistingGroupData(groupName) {
    if (!groupsData || !groupsData[groupName]) {
        return { substrings: [], fromAccounts: [], toAccounts: [] };
    }
    
    const group = groupsData[groupName];
    return {
        substrings: group.Substrings || [],
        fromAccounts: group.FromAccounts || [],
        toAccounts: group.ToAccounts || []
    };
}

function showCategoryActions(button) {
    const modal = document.getElementById("categoryActionsModal");
    if (!modal) return;

    // Remove highlight from previously selected row if exists
    if (currentTransaction) {
        currentTransaction.classList.remove("highlighted-row");
    }

    // Store and highlight the current transaction
    currentTransaction = button.closest("tr");
    currentTransaction.classList.add("highlighted-row");

    // Pre-fill the value input based on selected method
    updateValueField();

    // Use flex display to center the modal
    modal.style.display = "flex";
}

function handleByMethodChange() {
    updateValueField();
}

function updateValueField() {
    if (!currentTransaction) return;

    const byMethod = document.getElementById("categoryBy").value;
    const valueInput = document.getElementById("categorySubstring");

    if (byMethod === "substring") {
        valueInput.value = currentTransaction.querySelector("td:nth-child(5)").textContent.trim(); // Details
    } else if (byMethod === "fromAccount") {
        valueInput.value = currentTransaction.querySelector("td:nth-child(2)").textContent.trim(); // From Account
    } else if (byMethod === "toAccount") {
        valueInput.value = currentTransaction.querySelector("td:nth-child(3)").textContent.trim(); // To Account
    }
}

function submitCategoryAction(event) {
    event.preventDefault();

    if (!currentTransaction) return;

    const groupName = document.getElementById("groupSelect").value;
    const byMethod = document.getElementById("categoryBy").value;
    const value = document.getElementById("categorySubstring").value.trim();

    // Get existing group data
    const existingData = getExistingGroupData(groupName);

    // Prepare the updated data
    let data = {
        action: "upsertGroup",
        groupName: groupName,
        substrings: [...existingData.substrings],
        fromAccounts: [...existingData.fromAccounts],
        toAccounts: [...existingData.toAccounts]
    };

    // Add new rule based on selected method
    if (byMethod === "substring") {
        if (!data.substrings.includes(value)) {
            data.substrings.push(value);
        }
    } else if (byMethod === "fromAccount") {
        if (!data.fromAccounts.includes(value)) {
            data.fromAccounts.push(value);
        }
    } else if (byMethod === "toAccount") {
        if (!data.toAccounts.includes(value)) {
            data.toAccounts.push(value);
        }
    }

    fetch("/categorization", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
    })
        .then(() => window.location.reload())
        .catch((error) => console.error("Error:", error));

    closeCategoryModal();
}

function closeCategoryModal() {
    const modal = document.getElementById("categoryActionsModal");
    if (modal) {
        modal.style.display = "none";
        if (currentTransaction) {
            currentTransaction.classList.remove("highlighted-row");
            currentTransaction = null;
        }
    }
}

function openNewGroupModal(event) {
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }
    const modal = document.getElementById("newGroupModal");
    if (modal) {
        modal.style.display = "flex";
    }
}

function closeNewGroupModal() {
    const modal = document.getElementById("newGroupModal");
    if (modal) {
        modal.style.display = "none";
    }
}

// function updateTransactionsTable(response) {
//     // Hide any open modals
//     const categoryModal = document.getElementById("categoryActionsModal");
//     const newGroupModal = document.getElementById("newGroupModal");
//     if (categoryModal) {
//         categoryModal.style.display = "none";
//     }
//     if (newGroupModal) {
//         newGroupModal.style.display = "none";
//     }

//     // Update the groups dropdown with any new groups
//     const groupSelect = document.getElementById("groupSelect");
//     if (response.groups && groupSelect) {
//         groupSelect.innerHTML = '';
//         Object.keys(response.groups).forEach(groupName => {
//             const option = document.createElement('option');
//             option.value = groupName;
//             option.textContent = groupName;
//             groupSelect.appendChild(option);
//         });
//     }

//     // Always refresh the page after categorization to show updated transactions
//     window.location.reload();
// }
