
#include <stdio.h>

int myfunc() {
  return 1;
}

#include "myinc.h"

int main(int argc, char **argv) {
  printf("%d, %d\n", myfunc(), myfunc2());
  return 0;
}