document.addEventListener('DOMContentLoaded', function() {
    // Expenses vs Income Line Chart
    const expensesVsIncome = echarts.init(document.getElementById('expensesVsIncome'));
    expensesVsIncome.setOption({
        title: { text: 'Expenses vs Income' },
        xAxis: { type: 'category', data: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun'] },
        yAxis: { type: 'value' },
        series: [
            { name: 'Expenses', type: 'line', data: [1000, 1200, 900, 1500, 1300, 1100] },
            { name: 'Income', type: 'line', data: [1200, 1300, 1400, 1100, 1600, 1500] }
        ]
    });

    // Total Expenses Pie Chart
    const totalExpenses = echarts.init(document.getElementById('totalExpenses'));
    totalExpenses.setOption({
        title: { text: 'Total Expenses per Category' },
        series: [{
            type: 'pie',
            data: [
                { value: 1000, name: 'Housing' },
                { value: 800, name: 'Food' },
                { value: 600, name: 'Transport' },
                { value: 400, name: 'Entertainment' }
            ]
        }]
    });

    // Total Income Pie Chart
    const totalIncome = echarts.init(document.getElementById('totalIncome'));
    totalIncome.setOption({
        title: { text: 'Total Income per Category' },
        series: [{
            type: 'pie',
            data: [
                { value: 3000, name: 'Salary' },
                { value: 500, name: 'Investments' },
                { value: 200, name: 'Side Hustle' }
            ]
        }]
    });

    // Monthly Expenses Bar Chart
    const monthlyExpenses = echarts.init(document.getElementById('monthlyExpenses'));
    monthlyExpenses.setOption({
        title: { text: 'Monthly Expenses per Category' },
        tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
        legend: { data: ['Housing', 'Food', 'Transport', 'Entertainment'] },
        xAxis: { type: 'category', data: ['Mar 2024', 'Apr 2024', 'May 2024', 'Jun 2024'] },
        yAxis: { type: 'value' },
        series: [
            { name: 'Housing', type: 'bar', data: [500, 520, 510, 530] },
            { name: 'Food', type: 'bar', data: [400, 420, 410, 430] },
            { name: 'Transport', type: 'bar', data: [300, 310, 305, 315] },
            { name: 'Entertainment', type: 'bar', data: [200, 210, 205, 215] }
        ]
    });

    // Resize charts when window size changes
    window.addEventListener('resize', function() {
        expensesVsIncome.resize();
        totalExpenses.resize();
        totalIncome.resize();
        monthlyExpenses.resize();
    });
});
