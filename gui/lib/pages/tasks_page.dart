import 'package:flutter/material.dart';
import '../services/api_client.dart';

class TasksPage extends StatefulWidget {
  const TasksPage({super.key});

  @override
  State<TasksPage> createState() => _TasksPageState();
}

class _TasksPageState extends State<TasksPage> {
  final _api = ApiClient();
  bool _loading = false;
  List<String> _tasks = [];
  Map<String, dynamic>? _selectedTask;
  String? _error;

  // New pipeline form
  final _skillCtrl = TextEditingController(text: 'female_rebirth');
  final _trendCtrl = TextEditingController(text: '重生虐渣文持续霸榜');

  @override
  void dispose() {
    _skillCtrl.dispose();
    _trendCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadTasks() async {
    setState(() => _loading = true);
    try {
      _tasks = await _api.listTasks();
      _error = null;
    } catch (e) {
      _error = e.toString();
    }
    setState(() => _loading = false);
  }

  Future<void> _viewTask(String taskId) async {
    try {
      final detail = await _api.getTask(taskId);
      setState(() => _selectedTask = detail);
    } catch (e) {
      setState(() => _error = e.toString());
    }
  }

  Future<void> _runPipeline() async {
    setState(() => _loading = true);
    try {
      await _api.runPipeline(
        skillName: _skillCtrl.text,
        trendData: _trendCtrl.text,
      );
      await _loadTasks();
    } catch (e) {
      _error = e.toString();
    }
    setState(() => _loading = false);
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Row(
        children: [
          // Left: task list + new pipeline form
          Expanded(
            flex: 2,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Tasks', style: Theme.of(context).textTheme.headlineMedium),
                const SizedBox(height: 12),
                // New pipeline form
                Card(
                  color: const Color(0xFF181825),
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text('New Pipeline', style: TextStyle(fontWeight: FontWeight.bold)),
                        const SizedBox(height: 8),
                        TextField(
                          controller: _skillCtrl,
                          decoration: const InputDecoration(labelText: 'Skill', border: OutlineInputBorder()),
                        ),
                        const SizedBox(height: 8),
                        TextField(
                          controller: _trendCtrl,
                          decoration: const InputDecoration(labelText: 'Trend Data', border: OutlineInputBorder()),
                          maxLines: 2,
                        ),
                        const SizedBox(height: 12),
                        ElevatedButton.icon(
                          onPressed: _loading ? null : _runPipeline,
                          icon: const Icon(Icons.play_arrow),
                          label: const Text('Run Pipeline'),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    ElevatedButton.icon(
                      onPressed: _loadTasks,
                      icon: const Icon(Icons.refresh),
                      label: const Text('Refresh'),
                    ),
                  ],
                ),
                if (_error != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(_error!, style: const TextStyle(color: Color(0xFFF38BA8))),
                  ),
                const SizedBox(height: 12),
                Expanded(
                  child: _loading
                      ? const Center(child: CircularProgressIndicator())
                      : _tasks.isEmpty
                          ? const Center(child: Text('No tasks yet', style: TextStyle(color: Color(0xFF6C7086))))
                          : ListView.builder(
                              itemCount: _tasks.length,
                              itemBuilder: (ctx, i) => ListTile(
                                title: Text(_tasks[i]),
                                trailing: const Icon(Icons.chevron_right),
                                onTap: () => _viewTask(_tasks[i]),
                              ),
                            ),
                ),
              ],
            ),
          ),
          // Right: task detail
          const VerticalDivider(width: 32),
          Expanded(
            flex: 3,
            child: _selectedTask == null
                ? const Center(child: Text('Select a task to view details', style: TextStyle(color: Color(0xFF6C7086))))
                : _TaskDetail(task: _selectedTask!),
          ),
        ],
      ),
    );
  }
}

class _TaskDetail extends StatelessWidget {
  final Map<String, dynamic> task;
  const _TaskDetail({required this.task});

  @override
  Widget build(BuildContext context) {
    final stages = task['stages'] as List<dynamic>? ?? [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Task: ${task['task_id']}', style: Theme.of(context).textTheme.titleLarge),
        const SizedBox(height: 16),
        Expanded(
          child: ListView.builder(
            itemCount: stages.length,
            itemBuilder: (ctx, i) {
              final s = stages[i] as Map<String, dynamic>;
              final content = s['content'] as String? ?? '';
              final preview = content.length > 300 ? '${content.substring(0, 300)}...' : content;
              return Card(
                color: const Color(0xFF181825),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(s['stage'] as String? ?? '', style: const TextStyle(fontWeight: FontWeight.bold, color: Color(0xFFCBA6F7))),
                      const SizedBox(height: 8),
                      Text(preview, maxLines: 10, overflow: TextOverflow.ellipsis),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}
