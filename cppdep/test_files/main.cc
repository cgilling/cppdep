#include <stdio.h>

#include "alib.h"

int main(int argc, char** argv) {
#ifdef HELLO
  printf("Hello World!\n");
#else
  printf("%s\n", makeuuid().c_str());
#endif
  return 0;
}