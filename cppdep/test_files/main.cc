#include <stdio.h>

#include "alib.h"

int main(int argc, char** argv) {
#ifdef HELLO
  printf("Hello World!\n");
#else
  if (argc != 2) {
    printf("usage: %s gzfile\n", argv[0]);
    return 1;
  }
  printf("%s\n", gunzipPath(argv[1]).c_str());
#endif
  return 0;
}