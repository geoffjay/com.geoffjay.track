/// Measurements tab: latest values per metric (dashboard cards), the full
/// log, per-metric history, and the custom-metric catalog.
library;

import 'package:flutter/material.dart';

import '../api_client.dart';
import '../app_state.dart';
import '../models.dart';
import '../widgets.dart';

class MeasurementsScreen extends StatefulWidget {
  const MeasurementsScreen({super.key});

  @override
  State<MeasurementsScreen> createState() => _MeasurementsScreenState();
}

class _MeasurementsScreenState extends State<MeasurementsScreen> {
  int _epoch = 0;

  ApiClient get _api => AppStateScope.of(context).client!;

  void _refresh() => setState(() => _epoch++);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Measurements'),
        actions: [
          IconButton(
            icon: const Icon(Icons.category_outlined),
            tooltip: 'Metric catalog',
            onPressed: () => Navigator.of(context).push(MaterialPageRoute(
                builder: (_) => const MetricCatalogScreen())).then((_) => _refresh()),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _log,
        icon: const Icon(Icons.add),
        label: const Text('Log'),
      ),
      body: LoadableList<Measurement>(
        key: ValueKey('latest-$_epoch'),
        fetch: _api.latestMeasurements,
        emptyMessage: 'No measurements yet.\nTap Log to record your first one.',
        builder: (latest, _) => CustomScrollView(
          slivers: [
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
              sliver: SliverToBoxAdapter(
                child: Text('Latest',
                    style: Theme.of(context).textTheme.titleMedium),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              sliver: SliverGrid(
                gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                  maxCrossAxisExtent: 220,
                  mainAxisExtent: 92,
                  mainAxisSpacing: 12,
                  crossAxisSpacing: 12,
                ),
                delegate: SliverChildBuilderDelegate(
                  (context, i) {
                    final m = latest[i];
                    return Card(
                      child: InkWell(
                        borderRadius: BorderRadius.circular(12),
                        onTap: () => _openHistory(m.metricId, m.metricName),
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(m.metricName,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: Theme.of(context).textTheme.bodySmall),
                              const Spacer(),
                              Text('${fmtValue(m.value)} ${m.metricUnit}',
                                  style:
                                      Theme.of(context).textTheme.titleLarge),
                              Text(fmtDate(m.measuredAt),
                                  style: Theme.of(context).textTheme.bodySmall),
                            ],
                          ),
                        ),
                      ),
                    );
                  },
                  childCount: latest.length,
                ),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(16, 20, 16, 4),
              sliver: SliverToBoxAdapter(
                child: Text('All measurements',
                    style: Theme.of(context).textTheme.titleMedium),
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.only(bottom: 88),
              sliver: _MeasurementLog(
                api: _api,
                epoch: _epoch,
                onChanged: _refresh,
                metricId: null,
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _openHistory(int metricId, String name) {
    Navigator.of(context).push(MaterialPageRoute(
      builder: (_) => MetricHistoryScreen(
          metricId: metricId, metricName: name, onChanged: _refresh),
    ));
  }

  Future<void> _log() async {
    final metrics = await _api.listMetrics();
    if (!mounted) return;
    final result = await _logSheet(context, metrics);
    if (result == null) return;
    try {
      await _api.createMeasurement(
        metricId: result.$1,
        value: result.$2,
        measuredAt: result.$3,
        note: result.$4,
      );
      if (mounted) showSnack(context, 'Measurement logged');
    } on ApiException catch (e) {
      if (mounted) showSnack(context, e.message, error: true);
    }
    _refresh();
  }
}

/// Metric picker + value + optional backdated time + note.
/// Returns (metricId, value, measuredAt, note).
Future<(int, double, DateTime?, String)?> _logSheet(
    BuildContext context, List<Metric> metrics) {
  int? metricId = metrics.isNotEmpty ? metrics.first.id : null;
  final valueCtrl = TextEditingController();
  final noteCtrl = TextEditingController();
  DateTime? when;
  return showDialog<(int, double, DateTime?, String)>(
    context: context,
    builder: (dialogContext) => StatefulBuilder(
      builder: (dialogContext, setDialogState) => AlertDialog(
        title: const Text('Log measurement'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DropdownButtonFormField<int>(
              initialValue: metricId,
              decoration: const InputDecoration(
                  labelText: 'Metric', border: OutlineInputBorder()),
              items: metrics
                  .map((m) => DropdownMenuItem(
                      value: m.id,
                      child: Text(m.label.isNotEmpty ? m.label : m.name)))
                  .toList(),
              onChanged: (v) => setDialogState(() => metricId = v),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: valueCtrl,
              decoration: const InputDecoration(
                  labelText: 'Value', border: OutlineInputBorder()),
              keyboardType:
                  const TextInputType.numberWithOptions(decimal: true),
              autofocus: true,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: noteCtrl,
              decoration: const InputDecoration(
                  labelText: 'Note (optional)', border: OutlineInputBorder()),
            ),
            const SizedBox(height: 12),
            OutlinedButton.icon(
              onPressed: () async {
                final picked = await showDatePicker(
                  context: dialogContext,
                  initialDate: DateTime.now(),
                  firstDate: DateTime(2000),
                  lastDate: DateTime.now(),
                );
                if (picked != null) {
                  setDialogState(() => when = picked);
                }
              },
              icon: const Icon(Icons.calendar_today, size: 16),
              label: Text(
                  when == null ? 'Measured now' : 'Measured ${fmtDate(when)}'),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () {
              final v = double.tryParse(valueCtrl.text);
              if (metricId == null || v == null || v <= 0) {
                ScaffoldMessenger.of(dialogContext).showSnackBar(const SnackBar(
                    content: Text('Pick a metric and a positive value')));
                return;
              }
              Navigator.pop(
                  dialogContext, (metricId!, v, when, noteCtrl.text.trim()));
            },
            child: const Text('Save'),
          ),
        ],
      ),
    ),
  );
}

/// Edit (value + note) dialog; measured_at stays untouched server-side
/// (absent fields keep the stored timestamp on PATCH).
Future<void> editMeasurement(
    BuildContext context, ApiClient api, Measurement m,
    {required VoidCallback onChanged}) async {
  final valueCtrl = TextEditingController(text: fmtValue(m.value));
  final noteCtrl = TextEditingController(text: m.note);
  final saved = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text('Edit ${m.metricName}'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: valueCtrl,
            decoration: InputDecoration(
              labelText: 'Value (${m.metricUnit})',
              border: const OutlineInputBorder(),
            ),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            autofocus: true,
          ),
          const SizedBox(height: 12),
          TextField(
            controller: noteCtrl,
            decoration: const InputDecoration(
                labelText: 'Note', border: OutlineInputBorder()),
          ),
        ],
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('Cancel')),
        FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('Save')),
      ],
    ),
  );
  if (saved != true) return;
  final v = double.tryParse(valueCtrl.text);
  if (v == null || v <= 0) {
    if (context.mounted) {
      showSnack(context, 'Enter a positive number', error: true);
    }
    return;
  }
  try {
    await api.updateMeasurement(m.id, value: v, note: noteCtrl.text.trim());
    if (context.mounted) showSnack(context, 'Measurement updated');
  } on ApiException catch (e) {
    if (context.mounted) showSnack(context, e.message, error: true);
  }
  onChanged();
}

Future<void> deleteMeasurement(
    BuildContext context, ApiClient api, Measurement m,
    {required VoidCallback onChanged}) async {
  final confirmed = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: const Text('Delete measurement?'),
      content: Text(
          '${fmtValue(m.value)} ${m.metricUnit} on ${fmtDate(m.measuredAt)}'),
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
    await api.deleteMeasurement(m.id);
    if (context.mounted) showSnack(context, 'Deleted');
  } on ApiException catch (e) {
    if (context.mounted) showSnack(context, e.message, error: true);
  }
  onChanged();
}

/// Sliver list of measurements, optionally filtered to one metric. Mutations
/// report through [onChanged] so the owning screen refreshes.
class _MeasurementLog extends StatelessWidget {
  final ApiClient api;
  final int epoch;
  final VoidCallback onChanged;
  final int? metricId;

  const _MeasurementLog({
    required this.api,
    required this.epoch,
    required this.onChanged,
    required this.metricId,
  });

  @override
  Widget build(BuildContext context) {
    return LoadableList<Measurement>(
      key: ValueKey('log-$epoch-$metricId'),
      fetch: () => api.listMeasurements(metricId: metricId),
      emptyMessage: 'No measurements recorded.',
      builder: (items, _) => SliverList(
        delegate: SliverChildBuilderDelegate(
          (context, i) {
            final m = items[i];
            return ListTile(
              leading: const Icon(Icons.straighten),
              title: Text('${fmtValue(m.value)} ${m.metricUnit}'),
              subtitle: Text(
                '${m.metricName} · ${fmtDateTime(m.measuredAt)}'
                '${m.note.isNotEmpty ? ' · ${m.note}' : ''}',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              trailing: PopupMenuButton<String>(
                onSelected: (action) => action == 'edit'
                    ? editMeasurement(context, api, m, onChanged: onChanged)
                    : deleteMeasurement(context, api, m, onChanged: onChanged),
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Edit')),
                  PopupMenuItem(value: 'delete', child: Text('Delete')),
                ],
              ),
            );
          },
          childCount: items.length,
        ),
      ),
    );
  }
}

/// Per-metric history; supports the same edit/delete as the main log.
class MetricHistoryScreen extends StatefulWidget {
  final int metricId;
  final String metricName;
  final VoidCallback onChanged;

  const MetricHistoryScreen({
    super.key,
    required this.metricId,
    required this.metricName,
    required this.onChanged,
  });

  @override
  State<MetricHistoryScreen> createState() => _MetricHistoryScreenState();
}

class _MetricHistoryScreenState extends State<MetricHistoryScreen> {
  int _epoch = 0;

  @override
  Widget build(BuildContext context) {
    final api = AppStateScope.of(context).client!;
    return Scaffold(
      appBar: AppBar(title: Text(widget.metricName)),
      body: LoadableList<Measurement>(
        key: ValueKey('history-${widget.metricId}-$_epoch'),
        fetch: () => api.listMeasurements(metricId: widget.metricId),
        emptyMessage: 'Nothing recorded for this metric yet.',
        builder: (items, _) => ListView.builder(
          padding: const EdgeInsets.only(bottom: 24),
          itemCount: items.length,
          itemBuilder: (context, i) {
            final m = items[i];
            return ListTile(
              leading: const Icon(Icons.straighten),
              title: Text('${fmtValue(m.value)} ${m.metricUnit}'),
              subtitle: Text(
                  '${fmtDateTime(m.measuredAt)}'
                  '${m.note.isNotEmpty ? ' · ${m.note}' : ''}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis),
              trailing: PopupMenuButton<String>(
                onSelected: (action) {
                  void changed() {
                    setState(() => _epoch++);
                    widget.onChanged();
                  }

                  if (action == 'edit') {
                    editMeasurement(context, api, m, onChanged: changed);
                  } else {
                    deleteMeasurement(context, api, m, onChanged: changed);
                  }
                },
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Edit')),
                  PopupMenuItem(value: 'delete', child: Text('Delete')),
                ],
              ),
            );
          },
        ),
      ),
    );
  }
}

/// The metric catalog: list all, create/edit/delete custom metrics.
/// System metrics are locked (the API keeps them immutable).
class MetricCatalogScreen extends StatefulWidget {
  const MetricCatalogScreen({super.key});

  @override
  State<MetricCatalogScreen> createState() => _MetricCatalogScreenState();
}

class _MetricCatalogScreenState extends State<MetricCatalogScreen> {
  int _epoch = 0;

  void _refresh() => setState(() => _epoch++);

  @override
  Widget build(BuildContext context) {
    final api = AppStateScope.of(context).client!;
    return Scaffold(
      appBar: AppBar(title: const Text('Metrics')),
      floatingActionButton: FloatingActionButton(
        tooltip: 'New metric',
        onPressed: () async {
          final result = await _metricForm(context, null);
          if (result == null) return;
          try {
            await api.createMetric(
                name: result.$1,
                label: result.$2,
                unit: result.$3,
                category: result.$4);
            if (context.mounted) showSnack(context, 'Metric created');
          } on ApiException catch (e) {
            if (context.mounted) showSnack(context, e.message, error: true);
          }
          _refresh();
        },
        child: const Icon(Icons.add),
      ),
      body: LoadableList<Metric>(
        key: ValueKey('metrics-$_epoch'),
        fetch: api.listMetrics,
        emptyMessage: 'No metrics.',
        builder: (items, _) => ListView.builder(
          padding: const EdgeInsets.only(bottom: 88),
          itemCount: items.length,
          itemBuilder: (context, i) {
            final m = items[i];
            final label = m.label.isNotEmpty ? m.label : m.name;
            return ListTile(
              leading: Icon(m.category == 'vitals'
                  ? Icons.favorite_outline
                  : Icons.straighten),
              title: Text(label),
              subtitle: Text('${m.unit} · ${m.category}'),
              trailing: m.isSystem
                  ? const Tooltip(
                      message: 'System metric — immutable',
                      child: Icon(Icons.lock_outline))
                  : PopupMenuButton<String>(
                      onSelected: (action) async {
                        if (action == 'edit') {
                          final result = await _metricForm(context, m);
                          if (result == null) return;
                          try {
                            await api.updateMetric(m.id,
                                name: result.$1,
                                label: result.$2,
                                unit: result.$3,
                                category: result.$4);
                            if (context.mounted) {
                              showSnack(context, 'Metric updated');
                            }
                          } on ApiException catch (e) {
                            if (context.mounted) {
                              showSnack(context, e.message, error: true);
                            }
                          }
                        } else {
                          final ok = await showDialog<bool>(
                            context: context,
                            builder: (d) => AlertDialog(
                              title: const Text('Delete metric?'),
                              content: Text(
                                  '$label will be removed from the catalog.'),
                              actions: [
                                TextButton(
                                    onPressed: () => Navigator.pop(d, false),
                                    child: const Text('Cancel')),
                                FilledButton(
                                  onPressed: () => Navigator.pop(d, true),
                                  style: FilledButton.styleFrom(
                                      backgroundColor:
                                          Theme.of(d).colorScheme.error),
                                  child: const Text('Delete'),
                                ),
                              ],
                            ),
                          );
                          if (ok != true) return;
                          try {
                            await api.deleteMetric(m.id);
                            if (context.mounted) showSnack(context, 'Deleted');
                          } on ApiException catch (e) {
                            if (context.mounted) {
                              showSnack(context, e.message, error: true);
                            }
                          }
                        }
                        _refresh();
                      },
                      itemBuilder: (_) => const [
                        PopupMenuItem(value: 'edit', child: Text('Edit')),
                        PopupMenuItem(value: 'delete', child: Text('Delete')),
                      ],
                    ),
            );
          },
        ),
      ),
    );
  }
}

/// Create/edit form; returns (name, label, unit, category).
Future<(String, String, String, String)?> _metricForm(
    BuildContext context, Metric? existing) {
  final name = TextEditingController(text: existing?.name ?? '');
  final label = TextEditingController(text: existing?.label ?? '');
  final unit = TextEditingController(text: existing?.unit ?? '');
  final category = TextEditingController(text: existing?.category ?? '');
  return showDialog<(String, String, String, String)>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      title: Text(existing == null ? 'New metric' : 'Edit metric'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
              controller: name,
              decoration: const InputDecoration(
                  labelText: 'Name (slug)', border: OutlineInputBorder()),
              autofocus: true),
          const SizedBox(height: 12),
          TextField(
              controller: label,
              decoration: const InputDecoration(
                  labelText: 'Label', border: OutlineInputBorder())),
          const SizedBox(height: 12),
          TextField(
              controller: unit,
              decoration: const InputDecoration(
                  labelText: 'Unit', border: OutlineInputBorder())),
          const SizedBox(height: 12),
          TextField(
              controller: category,
              decoration: const InputDecoration(
                  labelText: 'Category', border: OutlineInputBorder())),
        ],
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('Cancel')),
        FilledButton(
          onPressed: () {
            if (name.text.trim().isEmpty || unit.text.trim().isEmpty) {
              ScaffoldMessenger.of(dialogContext).showSnackBar(const SnackBar(
                  content: Text('Name and unit are required')));
              return;
            }
            Navigator.pop(dialogContext, (
              name.text.trim(),
              label.text.trim(),
              unit.text.trim(),
              category.text.trim(),
            ));
          },
          child: const Text('Save'),
        ),
      ],
    ),
  );
}