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

    // Monthly Expenses Horizontal Stacked Bar Chart (Percentage)
    const monthlyExpenses = echarts.init(document.getElementById('monthlyExpenses'));
    const monthlyExpensesOption = {
        title: {
            text: 'Monthly Expenses per Category (%)',
            left: 'center',
            top: '5%'
        },
        tooltip: {
            trigger: 'axis',
            axisPointer: { type: 'shadow' },
            formatter: function(params) {
                const monthData = data.find(d => d.Start.startsWith(params[0].axisValue));
                let result = `${params[0].axisValue}<br>`;
                let total = 0;
                params.forEach(item => {
                    const value = parseMoneyString(monthData.Expense[item.seriesName]?.Total || '0');
                    total += value;
                    result += `${item.marker} ${item.seriesName}: ${value.toFixed(2)}<br>`;
                });
                result += `<strong>Total: ${total.toFixed(2)}</strong>`;
                return result;
            }
        },
        legend: {
            data: Array.from(expenseGroups),
            top: '10%'
        },
        toolbox: {
            feature: {
                saveAsImage: {},
                dataView: {}
            }
        },
        grid: {
            left: '3%',
            right: '4%',
            bottom: '3%',
            top: '20%',
            containLabel: true
        },
        xAxis: {
            type: 'value',
            name: 'Percentage',
            max: 100
        },
        yAxis: {
            type: 'category',
            data: labels.reverse()
        },
        series: Array.from(expenseGroups).map(group => ({
            name: group,
            type: 'bar',
            stack: 'total',
            emphasis: {
                focus: 'series'
            },
            data: data.map(stat => {
                let monthTotal = Object.values(stat.Expense).reduce((sum, expense) => sum + parseMoneyString(expense.Total), 0);
                let groupValue = parseMoneyString(stat.Expense[group]?.Total || '0');
                return (groupValue / monthTotal) * 100;
            }).reverse()
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
