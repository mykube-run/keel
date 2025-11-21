import { SchedulerClient } from './generated/scheduler_grpc_pb';
import { CreateTaskRequest } from './generated/scheduler_pb';

export class KeelSDK {
  private client: SchedulerClient;

  constructor(endpoint: string) {
    this.client = new SchedulerClient(endpoint, require('@grpc/grpc-js').credentials.createInsecure());
  }

  createTask(task: object) {
    const request = new CreateTaskRequest();
    // 实现任务创建逻辑
    return this.client.createTask(request);
  }
}