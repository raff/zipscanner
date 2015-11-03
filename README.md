# zipscanner
A zip file library that scan the file sequentially and tries to extract its content

This library implements a bufio.Scanner like interface that operates on a zip file. It tries to parse the zip file sequentially
in order to avoid seeks to the central directory (useful for streaming zip content from a network resource)
