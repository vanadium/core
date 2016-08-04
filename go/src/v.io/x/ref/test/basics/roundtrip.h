// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

struct rt_test {
  int server_sock;
  int client_sock;
};

int rt_init(struct rt_test* test);
int rt_sendn(struct rt_test test, int n);
int rt_recvn(struct rt_test test, int n);
void rt_stop(struct rt_test test);