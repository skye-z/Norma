#ifndef NORMA_TRAY_DARWIN_H
#define NORMA_TRAY_DARWIN_H

void normaTrayInit(const char *tooltip);
void normaTraySetIcon(const void *png, int len);
void normaTrayAddItem(const char *title, int tag);
void normaTrayAddSeparator(void);
void normaTraySetTitle(int tag, const char *title);

#endif
