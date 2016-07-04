// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

var dash = function() {
  // Data used to generate charts.
  var CHARTS = [
    {
      title: 'Latency (ms)',
      dataKey: 'Latency',
      id: 'latency'
    },
    {
      title: 'QPS (req/s)',
      dataKey: 'Qps',
      id: 'qps'
    },
    {
      title: 'CPU Usage (%)',
      dataKey: 'SysCPUUsagePct',
      id: 'cpu-usage-pct'
    },
    {
      title: 'Memory Usage (GB)',
      dataKey: 'SysMemUsageBytes',
      id: 'mem-usage-bytes',
      scale: 1/1024/1024/1024
    },
    {
      title: 'Disk Usage (GB)',
      dataKey: 'SysDiskUsageBytes',
      id: 'disk-usage-bytes',
      scale: 1/1024/1024/1024
    }
  ];

  var DURATIONS_TO_SECONDS = {
    '1h': 3600,
    '2h': 3600 * 2,
    '4h': 3600 * 4,
    '6h': 3600 * 6,
    '12h': 3600 * 12,
    '1d': 3600 * 24,
    '7d': 3600 * 24 * 7
  };

  var CHART_WIDTH = 700;
  var CHART_HEIGHT = 230;
  var CHART_COLOR = '#00838F';

  var durationInSeconds = 3600;
  var instance = '';

  function extractParameters() {
    var $url = $.url();
    instance = $url.param('instance');
    return instance !== undefined;
  }

  // Sets up handlers for changing duration.
  function setupDurationController() {
    $('#durations').children().click(function() {
      // Parse duration label and set durationInSeconds.
      durationInSeconds = DURATIONS_TO_SECONDS[$(this).text()];
      update();

      // Update UI.
      $('.duration-item.selected').toggleClass('selected');
      $(this).toggleClass('selected');
    });
  }

  // Gets data and updates charts.
  function update() {
    $('#loading-label').show();
    var params = {
      instance: instance,
      d: durationInSeconds
    };
    $('#error-msg').hide();
    $.ajax('stats?' + $.param(params))
        .done(function(data) {
          updateCharts(data);
        })
        .fail(function(){
          $('#error-msg').show();
        })
        .complete(function() {
          $('#loading-label').hide();
        });
  }

  // Updates all charts.
  function updateCharts(data) {
    CHARTS.forEach(function(chart) {
      var c = new
          google.visualization.AreaChart(document.getElementById(chart.id));
      var options = {
        title: chart.title,
        titleTextStyle: {
          fontSize: 14,
        },
        width: CHART_WIDTH,
        height: CHART_HEIGHT,
        legend: {
          position: 'none'
        },
        series: {
          0: {
            color: CHART_COLOR
          }
        },
        vAxis: {minValue: 0},
        hAxis: {
          minValue: new Date(data.MinTime * 1000),
          maxValue: new Date(data.MaxTime * 1000),
          gridlines: {
            count: -1,
            units: {
              days: {format: ['MMM dd']},
              hours: {format: ['h:mm a', 'h a']},
              milliseconds: {format: ['h:mm a']}
            },
          }
        }
      };
      c.draw(genDataTable(data[chart.dataKey], chart.scale), options);
    });
  }

  // Generates DataTable object from the given points.
  function genDataTable(points, scale) {
    var dt  = new google.visualization.DataTable();
    dt.addColumn('datetime', '');
    dt.addColumn('number', '');
    if (points) {
      dt.addRows(points.map(function(pt) {
        return [
            new Date(pt.Timestamp * 1000),
            scale ? pt.Value * scale : pt.Value
        ];
      }));
    } else {
      dt.addRow([new Date(), 0]);
    }
    return dt;
  }

  function init() {
    if (!extractParameters()) {
      alert('"n" is required for instance mounted name');
      return
    }
    $(function() {
      setupDurationController();
      update();
      // Update every minute.
      setInterval(update, 60000);
    });
  }

  return {
    init: init
  };
}();
