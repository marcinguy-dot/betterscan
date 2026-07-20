/* Joern / classic buffer overflow: unbounded gets() into fixed stack buffer. */
#include <stdio.h>
#include <string.h>

int main(void) {
    char buf[8];
    gets(buf); /* CWE-120 / CWE-242 */
    printf("%s\n", buf);
    return 0;
}

void copy_overflow(char *src) {
    char dest[16];
    strcpy(dest, src); /* CWE-120 */
}
