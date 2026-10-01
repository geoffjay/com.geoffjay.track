/// Fasts tab: active fast card with a live elapsed timer, start/end, and
/// the historical log with edit/delete.
library;

import 'dart:async';

import 'package:flutter/material.dart';

import '../api_client.dart';
import '../app_state.dart';
import '../models.dart';
import '../widgets.dart';

class FastsScreen extends StatefulWidget {
  const FastsScreen({super.key});

  @override
  State<FastsScreen> createState() => _FastsScreenState();
}

class _FastsScreenState extends State<FastsScreen> {
  int _epoch = 0;
  Timer? _ticker;

  ApiClient get _api => AppStateScope.of(context).client!;

  void _refresh() => setState(() => _epoch++);

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {

    return Scaffold(
      appBar: AppBar(title: const Text('Fasting')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _start,
        icon: const Icon(Icons.play_arrow),
        label: const Text('Start fast'),
      ),
      body: LoadableList<Fast>(
        key: ValueKey('fasts-$_epoch'),
        fetch: () async {
          final active = await _api.activeFast();
          final list = await _api.listFasts();
          return active == null
              ? list
              : [active, ...list.where((f) => f.id != active.id)];
        },
        emptyMessage:
            'No fasts yet.\nTap Start fast when you begin your next one.',
        builder: (fasts, _) {
          final active =
              fasts.where((f) => f.isActive).toList(growable: false);
          return ListView.builder(
            padding: const EdgeInsets.only(bottom: 88),
            itemCount: fasts.length + (active.isEmpty ? 0 : 0),
            itemBuilder: (context, i) {
              if (i == 0 && active.isNotEmpty) {
                return _ActiveFastCard(
                  fast: active.first,
                  onEnd: () => _end(active.first),
                  ticker: _refreshTick,
                );
              }
              final f = fasts[i - (i == 0 ? 0 : 0)];
              return _historyTile(f);
            },
          );
        },
      ),
    );
  }

  // Ticking re-render for the active card's elapsed counter.
  void _refreshTick() {
    if (mounted) setState(() {});
  }

  Widget _historyTile(Fast f) {
    if (f.isActive) return const SizedBox.shrink();
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: ListTile(
        leading: const Icon(Icons.timer_outlined),
        title: Text(f.durationHours == null
            ? 'Fast'
            : fmtDuration(f.durationHours!)),
        subtitle: Text(
            '${fmtDateTime(f.startedAt)} → ${fmtDateTime(f.endedAt)}'
            '${f.targetHours != null ? ' · target ${fmtDuration(f.targetHours!)}' : ''}'),
        trailing: PopupMenuButton<String>(
          onSelected: (action) =>
              action == 'edit' ? _edit(f) : _delete(f),
          itemBuilder: (_) => const [
            PopupMenuItem(value: 'edit', child: Text('Edit')),
            PopupMenuItem(value: 'delete', child: Text('Delete')),
          ],
        ),
      ),
    );
  }

  Future<void> _start() async {
    final result = await _fastForm(context, null);
    if (result == null) return;
    try {
      await _api.createFast(
          startedAt: result.$1, targetHours: result.$2, notes: result.$3);
      if (mounted) showSnack(context, 'Fast started');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _end(Fast f) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('End fast?'),
        content: Text(
            'Started ${fmtDateTime(f.startedAt)} — duration will be recorded as now.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('End fast'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await _api.endFast(f.id);
      if (mounted) showSnack(context, 'Fast ended');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _edit(Fast f) async {
    final result = await _fastForm(context, f);
    if (result == null) return;
    try {
      await _api.updateFast(f.id,
          startedAt: result.$1, targetHours: result.$2, notes: result.$3);
      if (mounted) showSnack(context, 'Fast updated');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }

  Future<void> _delete(Fast f) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Delete fast?'),
        content: Text('Started ${fmtDateTime(f.startedAt)}.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            style: FilledButton.styleFrom(
                backgroundColor: Theme.of(dialogContext).colorScheme.error),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await _api.deleteFast(f.id);
      if (mounted) showSnack(context, 'Deleted');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }
}

/// The prominent active-fast card with elapsed time ticking every second.
class _ActiveFastCard extends StatefulWidget {
  final Fast fast;
  final VoidCallback onEnd;
  final VoidCallback ticker;

  const _ActiveFastCard({
    required this.fast,
    required this.onEnd,
    required this.ticker,
  });

  @override
  State<_ActiveFastCard> createState() => _ActiveFastCardState();
}

class _ActiveFastCardState extends State<_ActiveFastCard> {
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final started = widget.fast.startedAt;
    final elapsed = started == null
        ? Duration.zero
        : DateTime.now().toUtc().difference(started);
    final h = elapsed.inHours;
    final m = elapsed.inMinutes % 60;
    final s = elapsed.inSeconds % 60;
    final target = widget.fast.targetHours;
    final progress = target == null
        ? null
        : (elapsed.inSeconds / (target * 3600)).clamp(0.0, 1.0);
    return Card(
      margin: const EdgeInsets.fromLTRB(12, 12, 12, 6),
      color: theme.colorScheme.primaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Icon(Icons.bolt, color: theme.colorScheme.onPrimaryContainer),
                const SizedBox(width: 8),
                Text('Fasting',
                    style: theme.textTheme.titleMedium?.copyWith(
                        color: theme.colorScheme.onPrimaryContainer)),
                const Spacer(),
                if (target != null)
                  Text('target ${fmtDuration(target)}',
                      style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onPrimaryContainer)),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              started == null
                  ? '—'
                  : '${h.toString().padLeft(2, '0')}:${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}',
              style: theme.textTheme.displaySmall?.copyWith(
                  color: theme.colorScheme.onPrimaryContainer,
                  fontFeatures: [const FontFeature.tabularFigures()]),
            ),
            if (progress != null) ...[
              const SizedBox(height: 8),
              LinearProgressIndicator(value: progress),
            ],
            if (widget.fast.notes.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(widget.fast.notes,
                  style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onPrimaryContainer)),
            ],
            const SizedBox(height: 12),
            FilledButton.icon(
              onPressed: widget.onEnd,
              icon: const Icon(Icons.stop),
              label: const Text('End fast'),
            ),
          ],
        ),
      ),
    );
  }
}

/// Start/edit form; returns (startedAt, targetHours, notes). Editing an
/// active fast keeps fields absent in the dialog unchanged.
Future<(DateTime?, double?, String)?> _fastForm(
    BuildContext context, Fast? existing) {
  final notesCtrl = TextEditingController(text: existing?.notes ?? '');
  final targetCtrl = TextEditingController(
      text: existing?.targetHours == null
          ? ''
          : fmtValue(existing!.targetHours!));
  DateTime? startedAt = existing?.startedAt;
  return showDialog<(DateTime?, double?, String)>(
    context: context,
    builder: (dialogContext) => StatefulBuilder(
      builder: (dialogContext, setDialogState) => AlertDialog(
        title: Text(existing == null ? 'Start fast' : 'Edit fast'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            OutlinedButton.icon(
              onPressed: () async {
                final date = await showDatePicker(
                  context: dialogContext,
                  initialDate: startedAt ?? DateTime.now(),
                  firstDate: DateTime(2000),
                  lastDate: DateTime.now(),
                );
                if (date == null || !dialogContext.mounted) return;
                final time = await showTimePicker(
                    context: dialogContext,
                    initialTime: TimeOfDay.fromDateTime(
                        startedAt ?? DateTime.now()));
                if (time == null) return;
                setDialogState(() {
                  startedAt = DateTime(
                      date.year, date.month, date.day, time.hour, time.minute);
                });
              },
              icon: const Icon(Icons.calendar_today, size: 16),
              label: Text(startedAt == null
                  ? 'Starting now'
                  : 'Started ${fmtDateTime(startedAt)}'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: targetCtrl,
              decoration: const InputDecoration(
                  labelText: 'Target hours (optional)',
                  border: OutlineInputBorder()),
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: notesCtrl,
              decoration: const InputDecoration(
                  labelText: 'Notes (optional)', border: OutlineInputBorder()),
            ),
          ],
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('Cancel')),
          FilledButton(
            onPressed: () {
              final t = double.tryParse(targetCtrl.text);
              Navigator.pop(
                  dialogContext, (startedAt, t, notesCtrl.text.trim()));
            },
            child: const Text('Save'),
          ),
        ],
      ),
    ),
  );
}