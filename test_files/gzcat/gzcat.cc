#include <zlib.h>

#include <stdio.h>

// prints out the contents of a gz file. Intended to work
// only with text.
int main(int argc, char** argv) {
  if (argc != 2) {
    printf("Usage: gzcat filepath\n");
    return 1;
  }

  const char* filepath = argv[1];

  gzFile fp = gzopen(filepath, "rb");

  if (fp == NULL) {
    printf("Failed to open file: %s\n", filepath);
    return 1;
  }

  char buf[1025];
  while(true) {
    int num = gzread(fp, buf, 1024);
    if (num < 0) {
      printf("Error reading gzip file: %s\n", filepath);
      return 1;
    } else if (num == 0) {
      break;
    } else {
      buf[num] = '\0';
      printf("%s", buf);
    }
  }

  gzclose(fp);

  return 0;
}