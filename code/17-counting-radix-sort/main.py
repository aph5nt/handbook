"""Counting sort and radix sort on the lesson's numbers: no two rows are ever compared.

Run: python3 main.py
"""


def counting_sort(days, k):
    count = [0] * k
    for day in days:                             # one pass: tally each key
        count[day] += 1
    start = [0] * k
    for d in range(1, k):                        # a running total: each key's first slot
        start[d] = start[d - 1] + count[d - 1]
    out = [None] * len(days)
    for day in days:                             # input order, so ties keep their order
        out[start[day]] = day
        start[day] += 1                          # the next row of this key goes one slot on
    return out, count


def first_slots(count):
    start, total = [], 0
    for c in count:
        start.append(total)
        total += c
    return start


def digit_of(number, d):
    return number // 10 ** d % 10


def stable_pass(numbers, d):
    buckets = [[] for _ in range(10)]            # ten buckets, one per digit
    for number in numbers:
        buckets[digit_of(number, d)].append(number)   # appended in order: stable
    return [number for bucket in buckets for number in bucket]


def unstable_pass(numbers, d):
    buckets = [[] for _ in range(10)]
    for number in numbers:
        buckets[digit_of(number, d)].insert(0, number)   # the bug: a tie goes in front
    return [number for bucket in buckets for number in bucket]


def radix_sort(numbers, digits):
    for d in range(digits):                      # lowest digit first
        numbers = stable_pass(numbers, d)
        print(f"  pass {d + 1}:", numbers)
    return numbers


def main():
    days = [3, 1, 4, 1, 0, 3, 2, 1]
    out, count = counting_sort(days, 5)
    print("counts:", count, " first slots:", first_slots(count))
    print("sorted:", out)
    print(f"a 64-bit key would need {2 ** 64:,} counters")

    print("radix sort:")
    radix_sort([170, 45, 75, 90, 802, 24, 2, 66], 3)

    ones = stable_pass([21, 12, 22, 11], 0)
    print("ones pass:", ones)
    print("stable tens pass:", stable_pass(ones, 1))
    print("unstable tens pass:", unstable_pass(ones, 1), "the ones order is lost")


if __name__ == "__main__":
    main()
