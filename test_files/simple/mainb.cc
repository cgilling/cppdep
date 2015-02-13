#include <stdio.h>

#include "a.h"
#include "mainb.h"

int main(int argc, char** argv) {
  printf("%s, %d\n", a(), mainb());
  return 0;
}
