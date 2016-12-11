// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

var tables = document.getElementsByClassName('mdl-data-table-sortable');
[].forEach.call(tables, makeTableSortable)

function makeTableSortable(tb) {
    var data = extractData();
    var headers = tb.getElementsByClassName('mdl-data-table__header-sortable');
    [].forEach.call(headers, makeColumnSortable)

    function makeColumnSortable(th, index) {
        appendSortIcon(th);
        th.isSortedAsc = th.classList.contains('mdl-data-table__header--sorted-ascending');
        var isnum = th.classList.contains('mdl-data-table__header-sortable-numeric');
        th.addEventListener('click', function() {
            var sortAsc = !th.isSortedAsc;
            [].forEach.call(headers, function reset(h) {
                h.classList.remove('mdl-data-table__header--sorted-ascending');
                h.classList.remove('mdl-data-table__header--sorted-descending');
                h.isSortedAsc = null;
            });
            th.classList.add(sortAsc ? 'mdl-data-table__header--sorted-ascending': 'mdl-data-table__header--sorted-descending');
            sortData(index, isnum, sortAsc);
            reorderRows();
            th.isSortedAsc = sortAsc;
        });
    }

    function sortData(index, isnum, ascending) {
        data.sort(function(a, b) {
            if(isnum) {
                var anum = Number(a.data[index]);
                var bnum = Number(b.data[index]);
                if(ascending) {
                        return anum-bnum;
                } else {
                        return bnum-anum;
                }
            }
            if(ascending) {
                return a.data[index].localeCompare(b.data[index]);
            } else {
                return b.data[index].localeCompare(a.data[index]);
            }
        });
    }

    function reorderRows() {
        for(var i = 0; i < data.length; i++ ) {
            tb.appendChild(data[i].row);
        }
        return data;
    }

    function appendSortIcon(th) {
        var icon = document.createElement('span');
        icon.className = 'mdl-data-table__sorticon';
        th.appendChild(icon);
    }
}

function extractData() {
  var rows = Array.prototype.slice.call(tables[0].rows);
  var data = [];
  for(var i = 1; i < rows.length; i++ ) {
      var row = rows[i];
      data[i-1] = {row: row, data: []};
      for(var j = 0; j < row.cells.length; j++ ) {
        var cell = row.cells[j];
        var target = cell.getElementsByClassName('mdl-data-table__cell-data')[0] || cell;
        data[i-1].data[j] = target.textContent;
      }
  }
  return data;
}
