/// Shared formatting helpers and small reusable widgets.
library;

import 'package:flutter/material.dart';

import 'models.dart';

/// Formats a value for display, trimming trailing zeros (82.5, not 82.50).
String fmtValue(double v) {
  var s = v.toStringAsFixed(2);
  s = s.replaceAll(RegExp(r'0+$'), '');
  if (s.endsWith('.')) s = s.substring(0, s.length - 1);
  return s;
}

/// "3h 25m" style duration from hours.
String fmtDuration(double hours) {
  final h = hours.floor();
  final m = ((hours - h) * 60).round();
  if (h == 0) return '${m}m';
  if (m == 0) return '${h}h';
  return '${h}h ${m}m';
}

/// "2h 15m" from seconds (timed exercise entries).
String fmtSeconds(double sec) {
  final h = sec ~/ 3600;
  final m = ((sec - h * 3600) / 60).round();
  if (h == 0) return '${m}m';
  if (m == 0) return '${h}h';
  return '${h}h ${m}m';
}

/// Short local date + time, e.g. "Sep 30, 7:30 AM".
String fmtDateTime(DateTime? t) {
  if (t == null) return '—';
  final local = t.toLocal();
  final hour = local.hour % 12 == 0 ? 12 : local.hour % 12;
  final ampm = local.hour >= 12 ? 'PM' : 'AM';
  return '${_monthAbbr(local.month)} ${local.day}, $hour:${local.minute.toString().padLeft(2, '0')} $ampm';
}

/// Short local date, e.g. "Sep 30".
String fmtDate(DateTime? t) {
  if (t == null) return '—';
  final local = t.toLocal();
  return '${_monthAbbr(local.month)} ${local.day}';
}

String _monthAbbr(int m) => const [
      'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec',
    ][m - 1];

/// Entry summary, e.g. "60 kg × 8" (weighted) or "30m, 5.2 km" (timed).
String entrySummary(WorkoutEntry e) {
  if (e.exerciseType == 'weighted') {
    final w = e.weightKg == null ? '—' : fmtValue(e.weightKg!);
    final r = e.reps == null ? '—' : '${e.reps}';
    return '$w kg × $r reps';
  }
  final d = e.durationSec == null ? '—' : fmtSeconds(e.durationSec!);
  if (e.distanceKm == null) return d;
  return '$d, ${fmtValue(e.distanceKm!)} km';
}

/// Snack bar helper: shows [message]; errors use the error color.
void showSnack(BuildContext context, String message, {bool error = false}) {
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(message),
      backgroundColor: error
          ? Theme.of(context).colorScheme.errorContainer
          : Theme.of(context).colorScheme.inverseSurface,
      behavior: SnackBarBehavior.floating,
    ),
  );
}

/// Fetches [fetch] into state and renders loading / error / empty / data.
/// Exposes [reload] for pull-to-refresh and post-mutation refreshes; call
/// it after create/update/delete so lists reflect server state.
class LoadableList<T> extends StatefulWidget {
  final Future<List<T>> Function() fetch;
  final Widget Function(List<T> items, void Function() reload) builder;
  final String emptyMessage;
  final Future<void> Function(void Function() reload)? onError;

  const LoadableList({
    super.key,
    required this.fetch,
    required this.builder,
    this.emptyMessage = 'Nothing here yet',
    this.onError,
  });

  @override
  State<LoadableList<T>> createState() => _LoadableListState<T>();
}

class _LoadableListState<T> extends State<LoadableList<T>> {
  List<T>? _items;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final items = await widget.fetch();
      if (!mounted) return;
      setState(() {
        _items = items;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      await widget.onError?.call(_load);
      if (!mounted) return;
      setState(() => _error = e);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('$_error', textAlign: TextAlign.center),
              const SizedBox(height: 16),
              FilledButton.tonal(
                onPressed: _load,
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      );
    }
    if (_items == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_items!.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            widget.emptyMessage,
            style: Theme.of(context).textTheme.bodyLarge,
            textAlign: TextAlign.center,
          ),
        ),
      );
    }
    return RefreshIndicator(onRefresh: _load, child: widget.builder(_items!, _load));
  }
}