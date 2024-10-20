document.addEventListener('DOMContentLoaded', function() {
    // Parse the JSON data
    const data = JSON.parse(document.getElementById('interval-statistics').textContent);

    console.log(data);

    const labels = data.map(stat => stat.Start.substring(0, 7)); // Format: YYYY-MM
    const incomeData = [];
    const expenseData = [];
    const incomeGroups = new Set();
    const expenseGroups = new Set();

    function parseMoneyString(str) {
        return parseFloat(str.replace(/\s/g, ''));
    }

    data.forEach(stat => {
        let totalIncome = 0;
        let totalExpense = 0;

        Object.entries(stat.Income).forEach(([group, data]) => {
            totalIncome += parseMoneyString(data.Total);
            incomeGroups.add(group);
        });

        Object.entries(stat.Expense).forEach(([group, data]) => {
            totalExpense += parseMoneyString(data.Total);
            expenseGroups.add(group);
        });

        incomeData.push(totalIncome);
        expenseData.push(totalExpense);
    });

    // Expenses vs Income Chart
    const expensesVsIncome = echarts.init(document.getElementById('expensesVsIncome'));
    const expensesVsIncomeOption = {
        title: { text: 'Expenses vs Income' },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'cross',
                label: {
                    backgroundColor: '#6a7985'
                }
            }
        },
        legend: {
            data: ['Expenses', 'Income']
        },
        toolbox: {
            feature: {
                saveAsImage: {},
                magicType: {
                    type: ['line', 'bar']
                },
                dataView: {}
            }
        },
        xAxis: {
            type: 'category',
            data: labels
        },
        yAxis: {
            type: 'value'
        },
        series: [
            { name: 'Expenses', type: 'line', data: expenseData },
            { name: 'Income', type: 'line', data: incomeData }
        ]
    };
    expensesVsIncome.setOption(expensesVsIncomeOption);

    // Total Expenses Bar Chart (previously Pie Chart)
    const totalExpenses = echarts.init(document.getElementById('totalExpenses'));
    const totalExpensesOption = {
        title: { text: 'Total Expenses per Category' },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            }
        },
        legend: {
            show: false // Hide legend as category names are on y-axis
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true
        },
        xAxis: {
            type: 'value',
            name: 'Amount'
        },
        yAxis: {
            type: 'category',
            data: Array.from(expenseGroups)
        },
        series: [{
            name: 'Expenses',
            type: 'bar',
            data: Array.from(expenseGroups).map(group => ({
                value: data.reduce((sum, stat) => sum + parseMoneyString(stat.Expense[group]?.Total || '0'), 0),
                name: group
            }))
        }]
    };
    totalExpenses.setOption(totalExpensesOption);

    // Total Income Bar Chart (previously Pie Chart)
    const totalIncome = echarts.init(document.getElementById('totalIncome'));
    const totalIncomeOption = {
        title: { text: 'Total Income per Category' },
        tooltip: {
            trigger: 'axis',
            axisPointer: {
                type: 'shadow'
            }
        },
        legend: {
            show: false // Hide legend as category names are on y-axis
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true
        },
        xAxis: {
            type: 'value',
            name: 'Amount'
        },
        yAxis: {
            type: 'category',
            data: Array.from(incomeGroups)
        },
        series: [{
            name: 'Income',
            type: 'bar',
            data: Array.from(incomeGroups).map(group => ({
                value: data.reduce((sum, stat) => sum + parseMoneyString(stat.Income[group]?.Total || '0'), 0),
                name: group
            }))
        }]
    };
    totalIncome.setOption(totalIncomeOption);

    // Monthly Expenses Bar Chart
    const monthlyExpenses = echarts.init(document.getElementById('monthlyExpenses'));
    const monthlyExpensesOption = {
        title: { text: 'Monthly Expenses per Category' },
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
        legend: { data: Array.from(expenseGroups) },
        toolbox: {
            feature: {
                saveAsImage: {},
                magicType: {
                    type: ['line', 'bar', 'stack']
                },
                dataView: {}
            }
        },
        xAxis: { type: 'category', data: labels },
        yAxis: { type: 'value' },
        series: Array.from(expenseGroups).map(group => ({
            name: group,
            type: 'bar',
            data: data.map(stat => parseMoneyString(stat.Expense[group]?.Total || '0'))
        }))
    };
    monthlyExpenses.setOption(monthlyExpensesOption);

    // Resize charts when window size changes
    window.addEventListener('resize', function() {
        expensesVsIncome.resize();
        totalExpenses.resize();
        totalIncome.resize();
        monthlyExpenses.resize();
    });
});
