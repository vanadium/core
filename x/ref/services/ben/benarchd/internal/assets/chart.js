// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

google.charts.load('current', {packages: ['corechart', 'line']});

function drawChart(items, elem) {
  var data = new google.visualization.DataTable();
  data.addColumn('datetime', 'date');
  data.addColumn('number', 'cpu time');

  data.addRows(items);

  var options = {
    hAxis: {
      title: 'date',
      slantedText: false,
    },
    vAxis: {
      title: 'ns/op'
    },
    backgroundColor: '#f1f8e9',
    width: '500',
    height: '190',
    explorer: {
      actions: ['dragToZoom', 'rightClickToReset'],
      keepInBounds: true
    },
  };

  var chart = new google.visualization.LineChart(elem);
  chart.draw(data, options);
}
