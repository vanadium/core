# Mounttable on Cloud Bigtable

This package and its sub-packages contain a [mounttable server] implementation
that uses Google's [Cloud Bigtable] service for storage. It is fast and scalable
to millions of nodes and, with enough replicas, millions of requests per second.

## Schema

Bigtable is not a relational database. Each table has only one key, and all
operations are atomic only at the row level. There is no way to mutate multiple
rows together atomically.

See the [Overview of Cloud Bigtable] for more information.

Our table has one row per node. The row key is a hash of the node name followed
by the name itself. This spreads rows evenly across all tablet servers with no
risk of name collision.

The table has three column families:

   * Metadata `m`: used to store information about the row:
      * ID `i`: A 4-byte ID for the node. Doesn't have to be globally unique.
      * Version `v`: The version changes every time the row is mutated. It is
        used to detect conflicts related to concurrent access.
      * Creator `c`: The cell's value is the name of its creator and its
        timestamp is the node creation time.
      * Sticky `s`: When this column is present, the node is not automatically
        garbage collected.
      * Permissions `p`: The [access.Permissions] of the node.
   * Servers `s`: Each mounted server has its own column. The column name is the
     server's address. The value contains the [mount flags]. The timestamp is
     the mount deadline.
   * Children `c`: Each child has its own column. The column name is the name of
     the child, without the path. The timestamp is the child creation time.

Example:

| Key              | ID  | Version | Creator | Sticky | Permissions  | Mounted Server...           | Child... |
| ---              | --- | ---     | ---     | ---    | ---          | ---                         | ---      |
| 540f1a56/        | id1 | 54321   | user    | 1      | {"Admin":... |                             | (id2)foo |
| 1234abcd/foo     | id2 | 123     | user    |        | {"Admin":... |                             | (id3)bar |
| 46d523e3/foo/bar | id3 | 5436    | user    |        | {"Admin":... | /example.com:123 (deadline) |          |

Counters are stored in another table, one row with one column per counter.

## Mutations

All operations use optimistic concurrency control. If a conflicting
change happens during a mutation, the whole operation is restarted.

  * Mutation on N
    * Get Node N
    * Check caller's permissions
    * Apply mutation on N if node version hasn't changed

If the node version changed, it means that another mutation was applied between
the time when we retrieved the node and when we tried to apply our mutation.
When that happens, we restart to whole operation, starting with retrieving the
node again.

### Mounting / Unmounting a server

Mounting a server consists of adding a new cell to the node's row. The column
family is `s`, the column name is the address of the server, the timestamp is
the mount deadline, and the value contains the [mount flags].

Unmounting a server consists of deleting the server's column.

### Adding / Removing a child node

Adding or removing a node requires two mutations: one on the parent, one on the
child.

When adding a node, we first add it to the parent, and then create a new row
with the same timestamp.

When deleting a node, we first delete the row, and then delete the child column
on the parent.

If the server process dies between the two mutations, it will leave the parent
with a reference to a child row that doesn't exist. As a consequence, the parent
will never be seen as "empty" and will not be automatically garbage collected.
This will be corrected when:

  * the child is re-created, or
  * the parent is forcibly deleted.

## Hot rows & caching

Some nodes are expected to be accessed significantly more than others, e.g. the
root node and its immediate children are traversed more often than nodes that
are further down the tree. The bigtable rows associated with these nodes are
"hotter" which can lead to traffic imbalance and poor performance.

This problem is alleviated with a small cache in the bigtable client. High
frequency or concurrent requests for the same rows can be bundled together to
reduce both latency and bigtable load at the same time.

## Opportunistic garbage collection

A node can be garbage-collected when it has no children, no mounted servers, and
hasn't been marked as _sticky_. A node is _sticky_ when someone explicitly
called [SetPermissions] on it.

The garbage collection happens opportunistically. When a mounttable server
accessed a node that is eligible for garbage collection while processing a
request, this node is removed before the ongoing request completes.

[Cloud Bigtable]: https://cloud.google.com/bigtable/docs/
[Overview of Cloud Bigtable]: https://cloud.google.com/bigtable/docs/api-overview
[mounttable server]: https://github.com/vanadium/go.v23/blob/master/services/mounttable/service.vdl
[SetPermissions]: https://github.com/vanadium/go.v23/blob/master/services/permissions/service.vdl#L58
[access.Permissions]: https://github.com/vanadium/go.v23/blob/master/security/access/types.vdl#L130
[mount flags]: https://github.com/vanadium/go.v23/blob/master/naming/types.vdl#L9
