#ifndef NORMA_WINDOW_DARWIN_H
#define NORMA_WINDOW_DARWIN_H

int normaWindowExists(const char *title);
void normaSetIgnoreMouse(const char *title, int ignore);
void normaSetFloating(const char *title, int floating);
void normaHideFromDock(void);
// 光标相对交互区域的位置：inside 为 1 表示在区域内，nx、ny 归一化到 [-1, 1]，y 向下为正；窗口不存在返回 0
int normaCursorInteract(const char *title, int *inside, double *nx, double *ny);

#endif
