// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "roundtrip.h"
#include <arpa/inet.h>
#include <netinet/in.h>
#include <stdio.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>

int rt_init(struct rt_test* test) {
  int err;
  int optval;
  int listen_sock;
  struct sockaddr_in server_addr;
  struct sockaddr_storage server_storage;
  socklen_t addr_size;

  listen_sock = socket(PF_INET, SOCK_STREAM, 0);

  server_addr.sin_family = AF_INET;
  server_addr.sin_port = htons(0);
  server_addr.sin_addr.s_addr = inet_addr("127.0.0.1");
  addr_size = sizeof(server_addr);
  memset(server_addr.sin_zero, '\0', sizeof(server_addr.sin_zero));

  err = bind(listen_sock, (struct sockaddr *) &server_addr, addr_size);
  if (err != 0) {
    return err;
  }

  err = listen(listen_sock, 5);
  if (err != 0) {
    return err;
  }

  // getsockname to find out the servers real port.
  if (getsockname(listen_sock, (struct sockaddr *) &server_addr, &addr_size) == -1) {
    return -1;
  }

  test->client_sock = socket(PF_INET, SOCK_STREAM, 0);
  err = connect(test->client_sock, (struct sockaddr *) &server_addr, addr_size);
  if (err != 0) {
    return err;
  }

  test->server_sock = accept(listen_sock, (struct sockaddr *) &server_storage, &addr_size);
  if (test->server_sock == -1) {
    return -1;
  }
  close(listen_sock);

  return 0;
}

void rt_stop(struct rt_test test) {
  close(test.server_sock);
  close(test.client_sock);
}

int rt_recvn(struct rt_test test, int n) {
  int i;
  char buffer[1];

  for (i = 0; i < n; i++) {
    while (1) {
      if (recv(test.server_sock, buffer, 1, 0) > 0) {
        break;
      }
    }
    if (send(test.server_sock, buffer, 1, 0) != 1) {
      return -1;
    }
  }
  return 0;
}

int rt_sendn(struct rt_test test, int n){
  int i;
  char buffer[1];

  for (i = 0; i < n; i++) {
    if (send(test.client_sock, buffer, 1, 0) != 1) {
      return -1;
    }
    while (1) {
      if (recv(test.client_sock, buffer, 1, 0) > 0) {
        break;
      }
    }
  }
  return 0;
}
