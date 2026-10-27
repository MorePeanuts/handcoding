# Any order that takes every course after its prerequisites is accepted. The
# reference returns [] exactly when the prerequisites have a cycle, so that
# is when the answer must be [].


def check(input, output, expected):
    n, prerequisites = input["numCourses"], input["prerequisites"]
    if not isinstance(output, list):
        return "the answer must be an array, got %r" % (output,)
    if expected == []:
        if output:
            return "the prerequisites have a cycle, so no order finishes every course: the answer is []"
        return None
    if not output:
        return "every course can be finished, e.g. in the order %s" % (expected,)
    if len(output) != n:
        return "the order has %d courses, but there are %d" % (len(output), n)
    position = {}
    for i, course in enumerate(output):
        if not isinstance(course, int) or not 0 <= course < n:
            return "%r is not a course (0 to %d)" % (course, n - 1)
        if course in position:
            return "course %d is taken twice" % course
        position[course] = i
    for a, b in prerequisites:
        if position[b] > position[a]:
            return "course %d comes before its prerequisite %d" % (a, b)
