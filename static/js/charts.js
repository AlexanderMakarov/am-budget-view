document.addEventListener('DOMContentLoaded', function() {
    const data = JSON.parse(document.getElementById('interval-statistics').textContent);

    console.log(data);

    const currencySelector = document.getElementById('currencySelector');
    let currentCurrency = currencySelector.value;
    
    function updateCharts(currency) {
        const currencyData = data.map(stat => stat[currency]);
        
        const labels = currencyData.map(stat => stat.Start.substring(0, 7)); // Format: YYYY-MM
        const incomeData = [];
        const expenseData = [];
        const incomeGroups = new Set();
        const expenseGroups = new Set();

        function parseMoneyString(str) {
            return parseFloat(str.replace(/\s/g, ''));
        }

        currencyData.forEach(stat => {
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

            incomeData.push(Number(totalIncome.toFixed(2)));
            expenseData.push(Number(totalExpense.toFixed(2)));
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
                { 
                    name: 'Expenses', 
                    color: 'red',
                    type: 'line', 
                    data: expenseData,
                },
                { 
                    name: 'Income', 
                    color: 'darkgreen',
                    type: 'line', 
                    data: incomeData 
                }
            ]
        };
        expensesVsIncome.setOption(expensesVsIncomeOption);

        // Total Expenses Bar Chart
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
            toolbox: {
                show: true,
                feature: {
                    saveAsImage: {},
                    dataView: {},
                    restore: {}
                }
            },
            grid: {
                left: '3%',
                right: '4%',
                bottom: '10%', // Increase bottom margin to accommodate axis name
                containLabel: true
            },
            xAxis: {
                type: 'value',
                name: 'Amount',
                nameLocation: 'middle',
                nameGap: 30,
                nameRotate: 0, // Keep it horizontal
                nameTextStyle: {
                    padding: [10, 0, 0, 0] // Add some padding to move it down
                }
            },
            yAxis: {
                type: 'category',
                data: Array.from(expenseGroups)
            },
            series: [{
                name: 'Expenses',
                type: 'bar',
                data: Array.from(expenseGroups).map(group => ({
                    value: currencyData.reduce((sum, stat) => Number((sum + parseMoneyString(stat.Expense[group]?.Total || '0')).toFixed(2)), 0),
                    name: group
                }))
            }]
        };
        totalExpenses.setOption(totalExpensesOption);

        // Total Income Bar Chart
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
            toolbox: {
                show: true,
                feature: {
                    saveAsImage: {},
                    dataView: {},
                    restore: {}
                }
            },
            grid: {
                left: '3%',
                right: '4%',
                bottom: '10%', // Increase bottom margin to accommodate axis name
                containLabel: true
            },
            xAxis: {
                type: 'value',
                name: 'Amount',
                nameLocation: 'middle',
                nameGap: 30,
                nameRotate: 0, // Keep it horizontal
                nameTextStyle: {
                    padding: [10, 0, 0, 0] // Add some padding to move it down
                }
            },
            yAxis: {
                type: 'category',
                data: Array.from(incomeGroups)
            },
            series: [{
                name: 'Income',
                type: 'bar',
                data: Array.from(incomeGroups).map(group => ({
                    value: currencyData.reduce((sum, stat) => Number((sum + parseMoneyString(stat.Income[group]?.Total || '0')).toFixed(2)), 0),
                    name: group
                }))
            }]
        };
        totalIncome.setOption(totalIncomeOption);

        // Store reversed labels once
        const reversedLabels = labels.slice().reverse();

        // Monthly Expenses Horizontal Stacked Bar Chart (Percentage)
        const monthlyExpenses = echarts.init(document.getElementById('monthlyExpenses'));
        const monthlyExpensesOption = {
            title: {
                text: 'Monthly Expenses per Category (%)',
                left: 'center',
                top: '10px'
            },
            tooltip: {
                trigger: 'item',
                axisPointer: { type: 'shadow' },
                formatter: function(params) {
                    const monthLabel = params.name;
                    let result = `${monthLabel}<br>`;
                    
                    // Get the month data to calculate absolute values
                    const monthData = currencyData[labels.length - 1 - labels.indexOf(monthLabel)];
                    const totals = monthData.Expense || monthData.Income;
                    const monthTotal = Object.values(totals).reduce((sum, data) => sum + parseMoneyString(data.Total), 0);

                    if (params.value > 0) {
                        const absoluteValue = (params.value * monthTotal / 100).toFixed(2);
                        result += `${params.marker} ${params.seriesName}: ${params.value.toFixed(2)}% (${absoluteValue})<br>`;
                    }
                    
                    return result;
                }
            },
            legend: {
                type: 'scroll',
                orient: 'horizontal',
                top: '40px',
                left: 'center',
                right: '10%',
            },
            toolbox: {
                feature: {
                    saveAsImage: {},
                    dataView: {}
                }
            },
            grid: {
                left: '3%',
                right: '5%',
                bottom: '5%',
                top: '100px',
                containLabel: true
            },
            xAxis: {
                type: 'value',
                name: 'Percentage',
                nameLocation: 'middle',
                nameGap: 30,
                max: 100,
                axisLabel: {
                    formatter: '{value}%'
                }
            },
            yAxis: {
                type: 'category',
                data: reversedLabels,
                axisLabel: {
                    interval: 0,
                    rotate: 0
                },
                axisTick: {
                    alignWithLabel: true
                }
            },
            series: Array.from(expenseGroups).map(group => ({
                name: group,
                type: 'bar',
                stack: 'total',
                emphasis: {
                    focus: 'series'
                },
                barCategoryGap: '30%',
                data: currencyData.map(stat => {
                    let monthTotal = Object.values(stat.Expense).reduce((sum, expense) => sum + parseMoneyString(expense.Total), 0);
                    let groupValue = parseMoneyString(stat.Expense[group]?.Total || '0');
                    return (groupValue / monthTotal) * 100;
                }).reverse()
            }))
        };

        function addChartClickHandler(chart, type) {
            chart.on('click', function(params) {
                if (params.seriesName && params.name) {
                    const month = params.name;  // Format: YYYY-MM
                    const group = params.seriesName;
                    window.location.href = `/transactions?month=${month}&group=${encodeURIComponent(group)}&type=${type}&currency=${currentCurrency}`;
                }
            });
        }

        monthlyExpenses.setOption(monthlyExpensesOption);
        addChartClickHandler(monthlyExpenses, 'expense');

        // Monthly Income Horizontal Stacked Bar Chart (Percentage)
        const monthlyIncome = echarts.init(document.getElementById('monthlyIncome'));
        const monthlyIncomeOption = {
            title: {
                text: 'Monthly Income per Category (%)',
                left: 'center',
                top: '10px'
            },
            tooltip: {
                trigger: 'item',
                axisPointer: { type: 'shadow' },
                formatter: function(params) {
                    const monthLabel = params.name;
                    let result = `${monthLabel}<br>`;
                    
                    // Get the month data to calculate absolute values
                    const monthData = currencyData[labels.length - 1 - labels.indexOf(monthLabel)];
                    const totals = monthData.Expense || monthData.Income;
                    const monthTotal = Object.values(totals).reduce((sum, data) => sum + parseMoneyString(data.Total), 0);

                    if (params.value > 0) {
                        const absoluteValue = (params.value * monthTotal / 100).toFixed(2);
                        result += `${params.marker} ${params.seriesName}: ${params.value.toFixed(2)}% (${absoluteValue})<br>`;
                    }
                    
                    return result;
                }
            },
            legend: {
                type: 'scroll',
                orient: 'horizontal',
                top: '40px',
                left: 'center',
                right: '10%',
            },
            toolbox: {
                feature: {
                    saveAsImage: {},
                    dataView: {}
                }
            },
            grid: {
                left: '3%',
                right: '5%',
                bottom: '5%',
                top: '100px',
                containLabel: true
            },
            xAxis: {
                type: 'value',
                name: 'Percentage',
                nameLocation: 'middle',
                nameGap: 30,
                max: 100,
                axisLabel: {
                    formatter: '{value}%'
                }
            },
            yAxis: {
                type: 'category',
                data: reversedLabels,
                axisLabel: {
                    interval: 0,
                    rotate: 0
                },
                axisTick: {
                    alignWithLabel: true
                }
            },
            series: Array.from(incomeGroups).map(group => ({
                name: group,
                type: 'bar',
                stack: 'total',
                emphasis: {
                    focus: 'series'
                },
                barCategoryGap: '30%',
                data: currencyData.map(stat => {
                    let monthTotal = Object.values(stat.Income).reduce((sum, income) => sum + parseMoneyString(income.Total), 0);
                    let groupValue = parseMoneyString(stat.Income[group]?.Total || '0');
                    return (groupValue / monthTotal) * 100;
                }).reverse()
            }))
        };

        monthlyIncome.setOption(monthlyIncomeOption);
        addChartClickHandler(monthlyIncome, 'income');

        // Resize charts when window size changes
        window.addEventListener('resize', function() {
            expensesVsIncome.resize();
            totalExpenses.resize();
            totalIncome.resize();
            monthlyExpenses.resize();
            monthlyIncome.resize();
        });
    }

    // Initialize charts with default currency
    updateCharts(currentCurrency);

    // Add currency selector change handler
    currencySelector.addEventListener('change', function(e) {
        currentCurrency = e.target.value;
        updateCharts(currentCurrency);
    });
});
